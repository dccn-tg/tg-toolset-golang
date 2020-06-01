package pdb

import (
	"fmt"
	"regexp"
	"time"
)

// Member defines the data structure of a pending role setting for a project member.
type Member struct {
	UserID string `json:"userID"`
	Role   string `json:"role"`
	// The `Timestamp` attribute is used for registering a timestamp concerning the
	// time the member role request is created. Currently, this is needed for PDBv1
	// to operate when cleaning up successfully performed pending roles in the SQL
	// database. This attribute is ignored for JSON (un)marshal.
	Timestamp time.Time `json:"-"`
}

// Storage defines the data structure for the storage resource of a project.
type Storage struct {
	QuotaGb int    `json:"quotaGb"`
	System  string `json:"system"`
}

// DataProjectProvision defines the data structure for sending project provision
// request to the filer-gateway.
type DataProjectProvision struct {
	ProjectID string   `json:"projectID"`
	Members   []Member `json:"members"`
	Storage   Storage  `json:"storage"`
}

// DataProjectUpdate defines the data structure for sending project update request
// to the filer-gateway.
type DataProjectUpdate struct {
	Members []Member `json:"members"`
	Storage Storage  `json:"storage"`
}

// User defines the data structure of a user in the project database.
type User struct {
	ID         string
	Firstname  string
	Middlename string
	Lastname   string
	Email      string
}

// Lab defines an enumerator for the lab categories.
type Lab int

// Set implements the interface for flag.Var().
func (l *Lab) Set(v string) error {
	switch v {
	case "EEG":
		*l = EEG
	case "MEG":
		*l = MEG
	case "MRI":
		*l = MRI
	default:
		return fmt.Errorf("unknown modality: %s", v)
	}
	return nil
}

// String implements the interface for flag.Var().  It returns the
// name of the lab modality.
func (l *Lab) String() string {
	s := "unknown"
	switch *l {
	case EEG:
		s = "EEG"
	case MEG:
		s = "MEG"
	case MRI:
		s = "MRI"
	}
	return s
}

// GetDescriptionRegex returns a regular expression pattern for the description of
// a modality.
func (l *Lab) GetDescriptionRegex() (*regexp.Regexp, error) {
	switch *l {
	case EEG:
		return regexp.MustCompile(".*(EEG).*"), nil
	case MEG:
		return regexp.MustCompile(".*(MEG).*"), nil
	case MRI:
		return regexp.MustCompile(".*(SKYRA|PRISMA(FIT){0,1}).*"), nil
	default:
		return nil, fmt.Errorf("unknown modality: %s", l.String())
	}
}

const (
	// EEG is a lab modality of the EEG labs.
	EEG Lab = iota
	// MEG is a lab modality of the MEG labs.
	MEG
	// MRI is a lab modality of the MRI labs.
	MRI
)

// LabBooking defines the data structure of a booking event in the lab calendar.
type LabBooking struct {
	// Project is the id of the project to which the experiment belongs.
	Project string
	// Subject is the subject id of the participant.
	Subject string
	// Session is the session id of the participant.
	Session string
	// Modality is the experiment modality name.
	Modality string
	// Operator is the user operating the experiment.
	Operator User
	// ProjectTitle is the title of the project.
	ProjectTitle string
	// StartTime is the time the experiment starts.
	StartTime time.Time
}
