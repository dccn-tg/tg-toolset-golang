package vol

import (
	"bytes"
	"io"
	"io/ioutil"
	"os/user"
	"path"
	"regexp"
	"strconv"

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

// Create provisions a project volume on the NetApp's ONTAP cluster filer.
func (m NetAppVolumeManager) Create(projecID string, quotaGiB int64) error {
	// TODO: create SSH command for the filer's management interface.

	session, err := m.connect()
	if err != nil {
		return err
	}
	defer session.Close()

	// filer manager command to list aggregates,
	// storage aggregate show -fields availsize,volcount -stat online
	var b bytes.Buffer
	session.Stdout = &b
	if err := session.Run("storage aggregate show -fields availsize,volcount -stat online"); err != nil {
		return err
	}

	// regex to parse output: regexp.Compile("^(aggr\S+)\s+(\S+[P,T,G,M,K]B)\s+([0-9]+)$")
	// - field 1: aggregation name
	// - field 2: free space
	// - field 3: number of volumes in the aggregate
	type aggregateInfo struct {
		name      string
		freeSpace uint64
		nvols     uint16
	}

	var aggregates []aggregateInfo
	reAggrInfo := regexp.MustCompile(`^(aggr\S+)\s+(\S+[P,T,G,M,K]B)\s+([0-9]+)$`)
	for {
		line, err := b.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if d := reAggrInfo.FindAllStringSubmatch(line, -1); d != nil {
			log.Debugf("aggregate: %s\n", line)
			var freeSpace, nvols int
			if freeSpace, err = strconv.Atoi(d[0][2]); err != nil {
				log.Debugf("cannot parse freespace of aggregate: %s\n", d[0][2])
				continue
			}
			if nvols, err = strconv.Atoi(d[0][3]); err != nil {
				log.Debugf("cannot parse nvols of aggregate: %s\n", d[0][3])
				continue
			}

			// TODO: convert freeSpace to unit of bytes

			aggregates = append(aggregates, aggregateInfo{
				name:      d[0][1],
				freeSpace: uint64(freeSpace),
				nvols:     uint16(nvols),
			})
		}
	}

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
