package pdb

import (
	"fmt"
	"regexp"
	"time"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
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

// map the corresponding modality id to a given `Lab`
func modality(lab Lab) *regexp.Regexp {
	switch lab {
	case MEG:
		return regexp.MustCompile("^meg.*")
	case EEG:
		return regexp.MustCompile("^eeg$")
	case MRI:
		return regexp.MustCompile("^mr[0-9.]t$")
	default:
		return regexp.MustCompile(".*")
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
	resp, err := api.GetProjects(v2.config)

	if err != nil {
		return nil, err
	}

	var projects []*Project
	for _, p := range resp.Projects {
		if activeOnly && projectStatusEnum(p.Status) != ProjectStatusActive {
			continue
		}
		projects = append(projects, &Project{
			ID:     p.Number,
			Name:   p.Title,
			Owner:  p.Owner.Username,
			Status: projectStatusEnum(p.Status),
		})
	}

	return projects, nil
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

	resp, err := api.GetUsers(v2.config)

	if err != nil {
		return nil, err
	}

	for _, u := range resp.Users {
		if u.Email == email {
			return &User{
				ID:         u.Username,
				Firstname:  u.FirstName,
				Middlename: u.MiddleName,
				Lastname:   u.LastName,
				Email:      u.Email,
				Status:     userStatusEnum(u.Status),
				Function:   userFunctionEnum(u.Function),
			}, nil
		}
	}

	return nil, fmt.Errorf("user not found, email: %s", email)
}

// GetLabBookings retrieves calendar bookings concerning the given `Lab` on a given `date` string.
// The `date` string is in the format of `2020-04-22`.
//
// Note that `Lab` has a rough definition in PDB1.  For PDB2, we need to map `Lab` more explicitly
// to resources via `Lab` -> modality -> resources.
func (v2 V2) GetLabBookings(lab Lab, date string) ([]*LabBooking, error) {

	start, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, err
	}

	// retrieve resources of given modalities corresponding to the `lab` type
	resources, err := api.GetLabs(
		v2.config,
		modality(lab),
	)

	log.Debugf("resources: %+v\n", resources)

	if err != nil {
		return nil, err
	}

	resp, err := api.GetBookingEvents(
		v2.config,
		resources,
		start,
		start.Add(24*time.Hour),
	)

	if err != nil {
		return nil, err
	}

	var bookings []*LabBooking
	for _, b := range resp.BookingEvents {

		if rsrc, err := api.LabResource(b.Resource); err == nil {
			bookings = append(bookings, &LabBooking{
				Project:      b.Booking.Project.Number,
				ProjectTitle: b.Booking.Project.Title,
				Operator: User{
					ID:         b.Booking.Owner.Username,
					Firstname:  b.Booking.Owner.FirstName,
					Middlename: b.Booking.Owner.MiddleName,
					Lastname:   b.Booking.Owner.LastName,
					Email:      b.Booking.Owner.Email,
					Status:     userStatusEnum(b.Booking.Owner.Status),
					Function:   userFunctionEnum(b.Booking.Owner.Function),
				},
				Modality:  rsrc.Id, // used resource ID as the Modality for PDB1 compatibility
				Subject:   b.Subject,
				Session:   b.Session,
				StartTime: b.Start.Local(),
			})
		}

	}

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
