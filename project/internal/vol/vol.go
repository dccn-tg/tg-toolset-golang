package vol

// VolumeManager defines functions for managing storage volume for a project.
type VolumeManager interface {
	// Create provisions a project volume on the targeting storage system.
	Create(quotaGiB int64) error
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
func (m NetAppVolumeManager) Create(quotaGiB int64) error {
	// TODO: create SSH command for the filer's management interface.
	return nil
}
