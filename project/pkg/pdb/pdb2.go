package pdb

import (
	"fmt"
	"regexp"
	"strings"
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

// GetLabBookingsForWorklist retrieves TENTATIVE and CONFIRMED calendar bookings concerning
// the given `Lab` on a given `date` string. The `date` string is in the format of `2020-04-22`.
func (v2 V2) GetLabBookingsForWorklist(lab Lab, date string) ([]*LabBooking, error) {
	loc, _ := time.LoadLocation("Local")

	dtime, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return nil, err
	}

	return v2.getLabBookingEvents(lab, dtime, dtime, true)
}

// GetLabBookingsForReport retrieves calendar bookings in all status concerning the given `Lab`
// in a date range of `[from, to]`. The `from` and `to` date strings are in the format of `2020-04-22`.
func (v2 V2) GetLabBookingsForReport(lab Lab, from, to string) ([]*LabBooking, error) {

	loc, _ := time.LoadLocation("Local")

	dfrom, err := time.ParseInLocation("2006-01-02", from, loc)
	if err != nil {
		return nil, err
	}
	dto, err := time.ParseInLocation("2006-01-02", to, loc)
	if err != nil {
		return nil, err
	}

	return v2.getLabBookingEvents(lab, dfrom, dto, false)
}

// getLabBookingEvents retrieves booking events from the core-api.
func (v2 V2) getLabBookingEvents(lab Lab, from, to time.Time, forWorklist bool) ([]*LabBooking, error) {

	log.Debugf("time range: %s ~ %s", from.String(), to.String())

	// retrieve resources of given modalities corresponding to the `lab` type
	resources, err := api.GetLabs(
		v2.config,
		modality(lab),
		true,
	)

	log.Debugf("resources: %+v\n", resources)

	if err != nil {
		return nil, err
	}

	resp, err := api.GetBookingEvents(
		v2.config,
		resources,
		from,
		to.Add(24*time.Hour),
	)

	if err != nil {
		return nil, err
	}

	var bookings []*LabBooking
	for _, b := range resp.BookingEvents {

		if forWorklist {
			if b.Status != api.BookingEventStatusConfirmed && b.Status != api.BookingEventStatusTentative {
				continue
			}
		}

		if rsrc, err := api.LabResource(b.Resource); err == nil {

			// fill empty subject and session ids for PDB1 compatibility
			_subId := b.Subject
			if _subId == "" {
				_subId = "Undefined"
			}

			_sesId := b.Session
			if _sesId == "" {
				_sesId = "1"
			}

			// get the name of the primary group, default is an empty string
			pg := ""
			for i, g := range b.Booking.Project.Owner.Groups {
				if i == 0 || g.Primary {
					pg = g.Group.Name
				}
			}

			bookings = append(bookings, &LabBooking{
				Project:       b.Booking.Project.Number,
				ProjectTitle:  b.Booking.Project.Title,
				Group:         pg,
				FundingSource: b.Booking.Project.FundingSource.Number,
				Operator: User{
					ID:         b.Booking.Owner.Username,
					Firstname:  b.Booking.Owner.FirstName,
					Middlename: b.Booking.Owner.MiddleName,
					Lastname:   b.Booking.Owner.LastName,
					Email:      b.Booking.Owner.Email,
					Status:     userStatusEnum(b.Booking.Owner.Status),
					Function:   userFunctionEnum(b.Booking.Owner.Function),
				},
				Lab:       strings.ToUpper(rsrc.Id), // used resource ID as the Lab for PDB1 compatibility
				Modality:  b.Booking.Experiment.Modality.ShortName,
				Subject:   _subId,
				Session:   _sesId,
				StartTime: b.Start.Local(),
				EndTime:   b.End.Local(),
				Status:    string(b.Status),
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
