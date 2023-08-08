package vol

import (
	"strconv"
	"strings"

	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
)

// VolumeManager defines functions for managing storage volume for a project.
type VolumeManager interface {

	// Config configures the VolumeManager with the given configuration.
	Config(config config.VolumeManagerConfiguration) error

	// Create provisions a project volume on the targeting storage system.
	Create(projectID string, quotaGiB int) error
}

// VolumeManagerMap defines a list of supported VolumeManager with associated path as key of
// the map.  The path is usually refers to the top-level mount point of the
// fileserver on which the VolumeManager performs actions.
var VolumeManagerMap = map[string]VolumeManager{
	"/project": NetAppVolumeManager{},
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
