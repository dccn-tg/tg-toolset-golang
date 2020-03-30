// Package filer defines the interfaces for provisioning or updating
// a storage space on DCCN storage systems (a.k.a. filer) for a user
// or a project.
package filer

import (
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

func init() {

	cfg := log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Info,
	}

	// initialize logger
	log.NewLogger(cfg, log.InstanceLogrusLogger)
}

// New function returns the corresponding File implementation based on the
// `system` name.
func New(system string) Filer {
	switch system {
	case "netapp":
		return NetApp{}
	case "freenas":
		return FreeNas{}
	default:
		return nil
	}
}

// Filer defines the interfaces for provisioning and setting storage space
// for a project and a personal home directory.
type Filer interface {
	CreateProject(projectID string, quotaGiB int) error
	CreateHome(username, groupname string, quotaGiB int) error
	SetProjectQuota(projectID string, quotaGiB int) error
	SetHomeQuota(username, groupname string, quotaGiB int) error
}
