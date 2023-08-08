// Package pdb defines and implments interfaces for interactions with the project database.
package pdb

import (
	"fmt"

	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
	log "github.com/dccn-tg/tg-toolset-golang/pkg/logger"
)

const Location = "Europe/Amsterdam"

func init() {

	cfg := log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Info,
	}

	// initialize logger
	log.NewLogger(cfg, log.InstanceLogrusLogger)
}

// New returns the `PDBClient` corresponding to the given
// PDB `version`.
func New(c config.PDBConfiguration) (PDB, error) {
	switch c.Version {
	case 1:
		return V1{config: c.V1}, nil
	case 2:
		return V2{config: c.V2}, nil
	default:
		return nil, fmt.Errorf("unknonw pdb version: %d", c.Version)
	}
}

// PDB defines the interface for various actions on project database.
type PDB interface {
	GetProjectPendingActions() (map[string]*DataProjectUpdate, error)
	DelProjectPendingActions(map[string]*DataProjectUpdate) error
	GetProjects(activeOnly bool) ([]*Project, error)
	GetUser(userID string) (*User, error)
	GetProject(projectID string) (*Project, error)
	GetUserByEmail(email string) (*User, error)
	GetLabBookingsForWorklist(lab Lab, date string) ([]*LabBooking, error)
	GetLabBookingsForReport(lab Lab, from, to string) ([]*LabBooking, error)
	GetExperimentersForSharedAnatomicalMR() ([]*User, error)
}
