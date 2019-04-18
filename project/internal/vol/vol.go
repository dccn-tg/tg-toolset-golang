package vol

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os/user"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

const (
	// VolumeMaxIOPS defines the maximum IOPS per volume for the QoS policy.
	VolumeMaxIOPS = 6000
	// VolumeUser defines the owner of a created volume on the NetApp filer.
	VolumeUser = "project"
	// VolumeGroup defines the group of a created volume on the NetApp filer.
	VolumeGroup = "project_g"
	// VolumeVserver defines the vserver of a created volume on the NetApp filer.
	VolumeVserver = "atreides"
	// VolumeJunctionPathRoot defines the base directory of the junction path of the volume.
	VolumeJunctionPathRoot = "/project"
)

// VolumeManager defines functions for managing storage volume for a project.
type VolumeManager interface {
	// Create provisions a project volume on the targeting storage system.
	Create(projectID string, quotaGiB int64) error
}

// NetAppVolumeManager implements VolumeManager interface specific for the NetApp's ONTAP cluster filer.
type NetAppVolumeManager struct {
	// AddressFilerMI is the hostname or ip address of the filer's management interface.
	AddressFilerMI string
}

// aggregateInfo is a data structure containing attributes of a NetApp ONTAP aggregate.
type aggregateInfo struct {
	name      string
	freeSpace uint64
	nvols     uint16
}

// byFreespace implements interface for sorting aggregates by freeSpace.
type byFreespace []aggregateInfo

func (b byFreespace) Len() int           { return len(b) }
func (b byFreespace) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byFreespace) Less(i, j int) bool { return b[i].freeSpace < b[j].freeSpace }

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

	// sort aggregates by freeSpace
	sort.Sort(byFreespace(aggregates))

	// print error message in debug mode
	// Question: shall the cmdErr be returned as an error?
	for _, line := range cmdErr {
		log.Debugln(line)
	}

	return
}

// hasQosPolicyGroup checks if the policy `policyGroupName` exists in the NetApp
// system.
func (m NetAppVolumeManager) hasQosPolicyGroup(policyGroupName string) (bool, error) {
	// filer manager command to list QoS policies
	var cmdOut, cmdErr []string
	cmdOut, cmdErr, err := m.execFilerMI("qos policy-group show -fields policy-group")

	if err != nil {
		return false, err
	}

	for _, line := range cmdOut {
		log.Debugln(line)
		if strings.TrimSpace(line) == policyGroupName {
			return true, nil
		}
	}

	// print error message in debug mode
	// Question: shall the cmdErr be returned as an error?
	for _, line := range cmdErr {
		log.Debugln(line)
	}

	return false, nil
}

// createQosPolicyGroup creates a QoS policy group on the NetApp filer with the given
// group name.
func (m NetAppVolumeManager) createQosPolicyGroup(policyGroupName string) error {
	// filer manager command to create a new QoS policy
	// fmt.Sprintf("qos policy-group create -policy-group %s -vserver atreides -max-throughput %diops", policyGroup, m.MaxIOPS)
	return nil
}

// createVolume creates a volume on the NetApp filer with the given volume name.
func (m NetAppVolumeManager) createVolume(volumeName string, quotaGiB int, aggregateName string,
	policyGroup string, junctionPath string) error {
	// cmd  = 'volume create -vserver atreides -volume %s -aggregate %s -size %s -user %s -group %s -junction-path %s' % (vol_name, g_aggr['name'], quota, ouid, ogid, fpath)
	// cmd += ' -security-style unix -unix-permissions 0750 -state online -autosize false -foreground true'
	// cmd += ' -policy dccn-projects -qos-policy-group %s -space-guarantee none -snapshot-policy none -type RW' % qos_policy_group
	// cmd += ' -percent-snapshot-space 0'
	return nil
}

// Create provisions a project volume on the NetApp's ONTAP cluster filer.
func (m NetAppVolumeManager) Create(projectID string, quotaGiB int) error {

	// get lists of aggregates sorted by free space
	aggregates, err := m.getAggregates()
	if err != nil {
		return err
	}

	if len(aggregates) < 1 {
		return fmt.Errorf("no aggregate available for creating volume")
	}

	// the aggregate selected is the one with the largest free space.
	aggr := aggregates[len(aggregates)-1]

	if aggr.freeSpace < uint64(quotaGiB*1000000000) {
		return fmt.Errorf("insufficient space on aggregate, required %d remaining %d", aggr.freeSpace, quotaGiB*1000000000)
	}
	log.Debugf("selected aggregate: %+v\n", aggr)

	// check and create policy group for volume specific QoS.
	// projectID --> policyGroup: 3010000.01 --> p3010000_01
	qosPolicyGroup := strings.Replace(fmt.Sprintf("p%s", projectID), ".", "_", -1)
	qosPolicyExist, err := m.hasQosPolicyGroup(qosPolicyGroup)
	if err != nil {
		return err
	}
	log.Debugf("found policy group %s: %t\n", qosPolicyGroup, qosPolicyExist)
	if !qosPolicyExist {
		err := m.createQosPolicyGroup(qosPolicyGroup)
		if err != nil {
			return err
		}
	}

	// create volume for project.
	// projectID --> volumeName: 3010000.01 --> project_3010000_01
	volumeName := strings.Replace(fmt.Sprintf("project_%s", projectID), ".", "_", -1)
	junctionPath := path.Join(VolumeJunctionPathRoot, projectID)

	err = m.createVolume(volumeName, quotaGiB, aggr.name, qosPolicyGroup, junctionPath)
	if err != nil {
		return err
	}

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

	isize, err := strconv.ParseFloat(sizeStr, 32)
	if err != nil {
		return 0, err
	}

	return uint64(isize * sizeUnitInBytes), nil
}
