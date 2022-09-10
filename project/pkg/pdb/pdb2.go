package pdb

import (
	"fmt"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"

	api "github.com/Donders-Institute/tg-toolset-golang/project/internal/pdb2"
)

// projectStatusEnum converts the project status string returned from the core-api
// to `ProjectStatus` enum.
func projectStatusEnum(status api.ProjectStatus) ProjectStatus {
	switch status {
	case api.ProjectStatusActive:
		return ProjectStatusActive
	case api.ProjectStatusInactive:
		return ProjectStatusInactive
	default:
		return ProjectStatusUnknown
	}
}

// userStatusEnum converts the user status string returned from the core-api
// to `UserStatus` enum.
func userStatusEnum(status api.UserStatus) UserStatus {
	switch status {
	case api.UserStatusTentative:
		return UserStatusTentative
	case api.UserStatusCheckedin:
		return UserStatusCheckedIn
	case api.UserStatusCheckedout:
		return UserStatusCheckedOut
	case api.UserStatusCheckedoutextended:
		return UserStatusCheckedOutExtended
	default:
		return UserStatusUnknown
	}
}

// userFunctionEnum converts the user function string returned from the core-api
// to `UserFunction` enum.
func userFunctionEnum(status api.UserFunction) UserFunction {
	switch status {
	case api.UserFunctionPrincipalinvestigator:
		return UserFunctionPrincipalInvestigator
	case api.UserFunctionResearchassistant:
		return UserFunctionResearchAssistant
	case api.UserFunctionResearchstaff:
		return UserFunctionResearchStaff
	case api.UserFunctionStaffscientist:
		return UserFunctionStaffScientist
	case api.UserFunctionPostdoctoralresearcher:
		return UserFunctionPostdoc
	case api.UserFunctionPhdstudent:
		return UserFunctionPhD
	case api.UserFunctionSupportingstaff:
		return UserFunctionSupportingStaff
	case api.UserFunctionOtherresearcher:
		return UserFunctionOtherResearcher
	case api.UserFunctionTrainee:
		return UserFunctionTrainee
	default:
		return UserFunctionUnknown
	}
}

// V2 implements interfaces of the new project database implemented with GraphQL-based core-api.
type V2 struct {
	config config.CoreAPIConfiguration
}

// DelProjectPendingActions performs deletion on the pending-role actions from the project
// database.
func (v2 V2) DelProjectPendingActions(actions map[string]*DataProjectUpdate) error {
	return fmt.Errorf("not implemented")
}

// GetProjectPendingActions performs queries to get project pending roles and project storage
// resource, and combines the results into a data structure that can be directly used for
// sending project update request to the filer-gateway API:
// https://github.com/Donders-Institute/filer-gateway
func (v2 V2) GetProjectPendingActions() (map[string]*DataProjectUpdate, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetProjects retrieves list of project identifiers from the project database.
func (v2 V2) GetProjects(activeOnly bool) ([]*Project, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetProject retrieves attributes of a project.
func (v2 V2) GetProject(projectID string) (*Project, error) {

	resp, err := api.GetProject(v2.config, projectID)

	if err != nil {
		return nil, err
	}

	return &Project{
		ID:     resp.Project.Number,
		Name:   resp.Project.Title,
		Owner:  resp.Project.Owner.Username,
		Status: projectStatusEnum(resp.Project.Status),
	}, nil
}

// GetUser gets the user identified by the given uid in the project database.
// It returns the pointer to the user data represented in the User data structure.
func (v2 V2) GetUser(uid string) (*User, error) {

	resp, err := api.GetUser(v2.config, uid)

	if err != nil {
		return nil, err
	}

	return &User{
		ID:         resp.User.Username,
		Firstname:  resp.User.FirstName,
		Middlename: resp.User.MiddleName,
		Lastname:   resp.User.LastName,
		Email:      resp.User.Email,
		Status:     userStatusEnum(resp.User.Status),
		Function:   userFunctionEnum(resp.User.Function),
	}, nil
}

// GetUserByEmail gets the user identified by the given email address.
func (v2 V2) GetUserByEmail(email string) (*User, error) {

	return nil, fmt.Errorf("not implemented")
}

// GetLabBookings retrieves calendar bookings concerning the given `Lab` on a given `date` string.
// The `date` string is in the format of `2020-04-22`.
func (v2 V2) GetLabBookings(lab Lab, date string) ([]*LabBooking, error) {
	bookings := make([]*LabBooking, 0)

	return bookings, nil
}

// GetExperimentersForSharedAnatomicalMR retrieves a list of experimenters that are
// allowed to access to the shared anatomical MR data at this moment.
//
// Those are experiments of projects that are conducting data acquisition using the
// EEG and MEG modalities.
func (v2 V2) GetExperimentersForSharedAnatomicalMR() ([]*User, error) {
	return nil, fmt.Errorf("not implemented")
}
