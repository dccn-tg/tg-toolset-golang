package vol

import (
	"bufio"
	"io"
	"io/ioutil"
	"os/user"
	"path"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// VolumeManager defines functions for managing storage volume for a project.
type VolumeManager interface {
	// Create provisions a project volume on the targeting storage system.
	Create(projectID string, quotaGiB int64) error
}

// NetAppVolumeManager implements VolumeManager interface specific for the NetApp's ONTAP cluster filer.
type NetAppVolumeManager struct {
	// FileSystemRoot is the ONTAP filesystem path under which the created project volume will be mounted.
	FileSystemRoot string
	// MaxIOPS is the maximum IOPS the volume should support and quarantee as a QoS.
	MaxIOPS int32
	// AddressFilerMI is the hostname or ip address of the filer's management interface.
	AddressFilerMI string
}

// aggregateInfo is a data structure containing attributes of a NetApp ONTAP aggregate.
type aggregateInfo struct {
	name      string
	freeSpace uint64
	nvols     uint16
}

// connect makes a SSH connection to the NetApp filer management interface and
// returns a ssh.Session.
func (m NetAppVolumeManager) connect() (session *ssh.Session, err error) {
	// get current user's SSH privat key $HOME/.ssh/id_rsa
	me, err := user.Current()
	if err != nil {
		return
	}

	key, err := ioutil.ReadFile(path.Join(me.HomeDir, ".ssh/id_rsa"))
	if err != nil {
		return
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return
	}

	config := &ssh.ClientConfig{
		User: "admin",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", m.AddressFilerMI, config)
	if err != nil {
		return
	}

	session, err = client.NewSession()
	if err != nil {
		return
	}

	return
}

// execFilerMI executes given filer command on the management interface remotely via SSH.
// It returns stdout and stderr as slice of strings, each contains the data of a line.
func (m NetAppVolumeManager) execFilerMI(cmd string) (stdout []string, stderr []string, err error) {
	session, err := m.connect()
	if err != nil {
		return
	}
	defer session.Close()

	var outReader, errReader io.Reader
	outReader, err = session.StdoutPipe()
	if err != nil {
		return
	}
	outScanner := bufio.NewScanner(outReader)

	errReader, err = session.StdoutPipe()
	if err != nil {
		return
	}
	errScanner := bufio.NewScanner(errReader)

	if err = session.Run(cmd); err != nil {
		return
	}

	for outScanner.Scan() {
		stdout = append(stdout, strings.TrimSpace(outScanner.Text()))
	}
	if err = outScanner.Err(); err != nil {
		return
	}

	for errScanner.Scan() {
		stderr = append(stderr, strings.TrimSpace(errScanner.Text()))
	}
	if err = errScanner.Err(); err != nil {
		return
	}

	return

}

// getAgreegates queries NetApp filer and returns a list of aggregates.
func (m NetAppVolumeManager) getAggregates() (aggregates []aggregateInfo, err error) {

	var cmdOut, cmdErr []string
	cmdOut, cmdErr, err = m.execFilerMI("storage aggregate show -fields availsize,volcount -stat online")

	if err != nil {
		return
	}

	// regex to parse output: regexp.Compile("^(aggr\S+)\s+(\S+[P,T,G,M,K]B)\s+([0-9]+)$")
	// - field 1: aggregation name
	// - field 2: free space
	// - field 3: number of volumes in the aggregate
	reAggrInfo := regexp.MustCompile(`^(aggr\S+)\s+(\S+[P,T,G,M,K]B)\s+([0-9]+)$`)

	var freeSpace uint64
	var nvols int
	var ierr error

	for _, line := range cmdOut {
		log.Debugln(line)
		if d := reAggrInfo.FindAllStringSubmatch(line, -1); d != nil {
			log.Debugf("aggregate: %s\n", line)

			if freeSpace, ierr = convertSize(d[0][2]); ierr != nil {
				log.Debugf("cannot parse freespace of aggregate: %s, reason: %s\n", d[0][2], ierr)
				continue
			}

			if nvols, ierr = strconv.Atoi(d[0][3]); ierr != nil {
				log.Debugf("cannot parse nvols of aggregate: %s, reason: %s\n", d[0][3], ierr)
				continue
			}

			aggregates = append(aggregates, aggregateInfo{
				name:      d[0][1],
				freeSpace: freeSpace,
				nvols:     uint16(nvols),
			})
		}
	}

	// print error message in debug mode
	// Question: shall the cmdErr be returned as an error?
	for _, line := range cmdErr {
		log.Debugln(line)
	}

	return
}

// hasQosPolicyGroup checks if the policy `policyGroupName` exists in the NetApp
// system.
func (m NetAppVolumeManager) hasQosPolicyGroup(policyGroupName string) bool {

	return false
}

// Create provisions a project volume on the NetApp's ONTAP cluster filer.
func (m NetAppVolumeManager) Create(projecID string, quotaGiB int) error {

	aggregates, err := m.getAggregates()
	if err != nil {
		return err
	}

	log.Debugf("aggregates: %+v\n", aggregates)

	// filer manager command to list QoS policies
	// qos policy-group show

	// filer manager command to create a new QoS policy
	//
	// projectID --> policyGroup: 3010000.01 --> p3010000_01
	// fmt.Sprintf("qos policy-group create -policy-group %s -vserver atreides -max-throughput %diops", policyGroup, m.MaxIOPS)

	// filer manager command to create project volume
	//
	// projectID --> volumeName: 3010000.01 --> project_3010000_01

	// cmd  = 'volume create -vserver atreides -volume %s -aggregate %s -size %s -user %s -group %s -junction-path %s' % (vol_name, g_aggr['name'], quota, ouid, ogid, fpath)
	// cmd += ' -security-style unix -unix-permissions 0750 -state online -autosize false -foreground true'
	// cmd += ' -policy dccn-projects -qos-policy-group %s -space-guarantee none -snapshot-policy none -type RW' % qos_policy_group
	// cmd += ' -percent-snapshot-space 0'

	return nil
}

// convertSize parses the size string and convert it into bytes in integer
func convertSize(sizeStr string) (uint64, error) {

	sizeUnitInBytes := 1.

	switch {
	case strings.HasSuffix(sizeStr, "KB"):
		sizeStr = strings.TrimSuffix(sizeStr, "KB")
		sizeUnitInBytes = 1000.
	case strings.HasSuffix(sizeStr, "MB"):
		sizeStr = strings.TrimSuffix(sizeStr, "MB")
		sizeUnitInBytes = 1000000.
	case strings.HasSuffix(sizeStr, "GB"):
		sizeStr = strings.TrimSuffix(sizeStr, "GB")
		sizeUnitInBytes = 1000000000.
	case strings.HasSuffix(sizeStr, "TB"):
		sizeStr = strings.TrimSuffix(sizeStr, "TB")
		sizeUnitInBytes = 1000000000000.
	case strings.HasSuffix(sizeStr, "PB"):
		sizeStr = strings.TrimSuffix(sizeStr, "PB")
		sizeUnitInBytes = 1000000000000000.
	default:
	}

	if isize, err := strconv.ParseFloat(sizeStr, 32); err != nil {
		return 0, err
	} else {
		return uint64(isize * sizeUnitInBytes), nil
	}
}
