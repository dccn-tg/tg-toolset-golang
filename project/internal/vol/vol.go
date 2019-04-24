package vol

import (
	"strconv"
	"strings"
)

// VolumeManager defines functions for managing storage volume for a project.
type VolumeManager interface {
	// Create provisions a project volume on the targeting storage system.
	Create(projectID string, quotaGiB int64) error
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
