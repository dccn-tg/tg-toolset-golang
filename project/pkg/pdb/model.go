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

// StorageInfo defines the data structure for the storage resource information of a project,
// including the actual storage usage.
type StorageInfo struct {
	QuotaGb int    `json:"quotaGb"`
	UsageMb int    `json:"usageMb"`
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

// DataProjectInfo defines the data structure for received project storage information
// returned from the filer-gateway.
type DataProjectInfo struct {
	ProjectID string      `json:"projectID"`
	Members   []Member    `json:"members"`
	Storage   StorageInfo `json:"storage"`
}

// Project defines the data structure of a project in the project database.
type Project struct {
	ID     string        `json:"projectID"`
	Name   string        `json:"projectName"`
	Owner  string        `json:"owner"`
	Status ProjectStatus `json:"status"`
	Start  time.Time     `json:"start"`
	End    time.Time     `json:"end"`
}

// ProjectStatus defines PDB project status.
type ProjectStatus int

const (
	// ProjectStatusUnknown refers to unexpected/unknown project status in PDB.
	ProjectStatusUnknown ProjectStatus = iota - 1
	// ProjectStatusActive refers to active project.
	ProjectStatusActive
	// ProjectStatusInactive refers to inactive project.
	ProjectStatusInactive
)

// User defines the data structure of a user in the project database.
type User struct {
	ID         string       `json:"userID"`
	Firstname  string       `json:"firstName"`
	Middlename string       `json:"middleName"`
	Lastname   string       `json:"lastName"`
	Email      string       `json:"email"`
	Status     UserStatus   `json:"status"`
	Function   UserFunction `json:"function"`
}

// DisplayName constructs the user's display name using `Firstname`, `Middlename` and `Lastname`.
func (u User) DisplayName() string {
	if u.Middlename != "" {
		return fmt.Sprintf("%s %s %s", u.Firstname, u.Middlename, u.Lastname)
	} else {
		return fmt.Sprintf("%s %s", u.Firstname, u.Lastname)
	}
}

// UserFunction defines PDB user function.
// TODO: refine the fine-grained user functions.
type UserFunction int

const (
	// UserFunctionOther for other functions not indicated below.
	UserFunctionOther UserFunction = iota - 1
	// UserFunctionPrincipalInvestigator for users with the principle investigators function.
	UserFunctionPrincipalInvestigator
	// UserFunctionTrainee for users that are trainees.
	UserFunctionTrainee
	// UserFunctionPhD for users that are PhD students.
	UserFunctionPhD
	// UserFunctionPostdoc for users that are Postdocs.
	UserFunctionPostdoc
	// UserFunctionResearchSupport for reseache support.
	UserFunctionResearchSupport
	// UserFunctionOtherSupport for other support staffs.
	UserFunctionOtherSupport
	// UserFunctionSupportingStaff for supporting staffs.
	UserFunctionSupportingStaff
	// UserFunctionResearchStaff for research staffs.
	UserFunctionResearchStaff
	// UserFunctionResearchAssistant for research assistant.
	UserFunctionResearchAssistant
	// UserFunctionStaffScientist for ataff scientist.
	UserFunctionStaffScientist
	// UserFunctionOtherResearcher for general researchers.
	UserFunctionOtherResearcher
	// UserFunctionSeniorResearcher for senior researchers.
	UserFunctionSeniorResearcher
	// UserFunctionUnknown for unknown/unexpected user function.
	UserFunctionUnknown
)

// UserStatus defines PDB user status.
type UserStatus int

const (
	// UserStatusUnknown refers to unexpected/unknown user status in PDB.
	UserStatusUnknown UserStatus = iota - 1
	// UserStatusCheckedIn refers to the status when the user is checked in.
	UserStatusCheckedIn
	// UserStatusCheckedOut refers to the status when the user has checked out.
	UserStatusCheckedOut
	// UserStatusCheckedOutExtended refers to the status when the user applied extended checkout.
	UserStatusCheckedOutExtended
	// UserStatusTentative refers to the status when the user is registered by not yet checked-in after following certain procedure.
	UserStatusTentative
)

// String implements the interface for `fmt.Stringer`.  It returns the
// human-readable name of the state.
func (u UserStatus) String() string {
	s := "Unknown"
	switch u {
	case UserStatusCheckedIn:
		s = "CheckedIn"
	case UserStatusCheckedOut:
		s = "CheckedOut"
	case UserStatusTentative:
		s = "Tentative"
	case UserStatusCheckedOutExtended:
		s = "CheckedOutExtended"
	}
	return s
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
	case "ALL":
		*l = ALL
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
	case ALL:
		s = "ALL"
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
	case ALL:
		return regexp.MustCompile(".*"), nil
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
	// all lab modalities
	ALL
)

// LabBooking defines the data structure of a booking event in the lab calendar.
type LabBooking struct {
	// Project is the id of the project to which the experiment belongs.
	Project string `json:"project_id"`
	// FundingSource is the number of the project funding source.
	FundingSource string `json:"fundingSource"`
	// Group is the name of the primary group of the project owner.
	Group string `json:"group"`
	// Subject is the subject id of the participant.
	Subject string `json:"subject"`
	// Session is the session id of the participant.
	Session string `json:"session"`
	// Lab is the lab name
	Lab string `json:"lab"`
	// Modality is the experiment modality name.
	Modality string `json:"modality"`
	// Operator is the user operating the experiment.
	Operator User `json:"operator"`
	// ProjectTitle is the title of the project.
	ProjectTitle string `json:"project_title"`
	// Status is the status of the booking.
	Status string `json:"status"`
	// StartTime is the time the experiment starts.
	StartTime time.Time `json:"start_time"`
	// EndTime is the time the experiment ends.
	EndTime time.Time `json:"end_time"`
}

// OpsIgnored is a specific error referring ignored operation.
type OpsIgnored struct {
	// Message is the detail information of the ignored operation.
	Message string
}

func (e *OpsIgnored) Error() string {
	return e.Message
}

// OoqLastAlert is the internal data structure.
type OoqLastAlert struct {
	// Timestamp is the moment the alert was sent.
	Timestamp time.Time
	// UsagePercent is the storage usage ratio in percent at the moment the alert was sent.
	UsagePercent int
	// UsagePercentLastCheck is the storage usage ratio at the last check.
	UsagePercentLastCheck int
}

// OotLastAlert is the internal data structure.
type OotLastAlert struct {
	// Timestamp is the moment the alert was sent.
	Timestamp time.Time
}
