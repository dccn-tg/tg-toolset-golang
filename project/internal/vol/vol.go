package vol

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

// Create provisions a project volume on the NetApp's ONTAP cluster filer.
func (m NetAppVolumeManager) Create(projecID string, quotaGiB int64) error {
	// TODO: create SSH command for the filer's management interface.

	// filer manager command to list aggregates,
	// storage aggregate show -fields availsize,volcount -stat online
	//
	// regex to parse output: regexp.Compile("^(aggr\S+)\s+(\S+[P,T,G,M,K]B)\s+([0-9]+)$")
	// - field 1: aggregation name
	// - field 2: free space
	// - field 3: number of volumes in the aggregate

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
