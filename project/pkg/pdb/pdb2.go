package pdb

import (
	"fmt"

	"github.com/shurcooL/graphql"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"
)

var (
	// action2role maps the pending role action string of the Core API
	// (e.g. `SetToXYZ`) to the string representation of the `acl.Role`,
	// the value can be used directly for the API of the filer-gateway:
	// https://github.com/Donders-Institute/filer-gateway
	//
	// One exception is that the action `Unset` is mapped to `none` which
	// is not defined as a `acl.Role`.
	action2role map[string]string = map[string]string{
		"SetToManager":     acl.Manager.String(),
		"SetToContributor": acl.Contributor.String(),
		"SetToViewer":      acl.Viewer.String(),
		"Unset":            "none",
	}
)

// V2 implements interfaces of the new project database implemented with GraphQL-based core-api.
type V2 struct {
	config config.CoreAPIConfiguration
}

// DelProjectPendingActions performs deletion on the pending-role actions from the project
// database.
func (v2 V2) DelProjectPendingActions(actions map[string]*DataProjectUpdate) error {

	pendingRoles := make(map[string][]Member)

	for pid, data := range actions {
		pendingRoles[pid] = data.Members
	}

	return delProjectPendingRoles(v2.config, pendingRoles)
}

// GetProjectPendingActions performs queries to get project pending roles and project storage
// resource, and combines the results into a data structure that can be directly used for
// sending project update request to the filer-gateway API:
// https://github.com/Donders-Institute/filer-gateway
func (v2 V2) GetProjectPendingActions() (map[string]*DataProjectUpdate, error) {
	actions := make(map[string]*DataProjectUpdate)

	pendingRoles, err := getProjectPendingRoles(v2.config)
	if err != nil {
		return actions, err
	}

	for pid, members := range pendingRoles {
		stor, err := getProjectStorageResource(v2.config, pid)
		if err != nil {
			log.Errorf("%s", err)
		}
		actions[pid] = &DataProjectUpdate{
			Members: members,
			Storage: *stor,
		}
	}

	return actions, nil
}

// GetProjects retrieves list of project identifiers from the project database.
func (v2 V2) GetProjects(activeOnly bool) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetUser gets the user identified by the given uid in the project database.
// It returns the pointer to the user data represented in the User data structure.
func (v2 V2) GetUser(uid string) (*User, error) {

	return &User{}, nil
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

// getProjectStorageResource retrieves the storage resource of a given project.
func getProjectStorageResource(conf config.CoreAPIConfiguration, projectID string) (*Storage, error) {
	var stor Storage

	// GraphQL query construction
	var qry struct {
		Project struct {
			QuotaGb graphql.Int
		} `graphql:"project(number: $id)"`
	}

	vars := map[string]interface{}{
		"id": graphql.ID(projectID),
	}

	if err := query(conf.AuthClientSecret, conf.AuthURL, conf.CoreAPIURL, &qry, vars); err != nil {
		log.Errorf("fail to query project quota: %s", err)
		return nil, err
	}

	// TODO: do not hardcode system to "netapp".
	stor = Storage{
		QuotaGb: int(qry.Project.QuotaGb),
		System:  "netapp",
	}

	return &stor, nil
}

// getProjectPendingRoles retrieves a list of pending actions concering the project
// member roles.
func getProjectPendingRoles(conf config.CoreAPIConfiguration) (map[string][]Member, error) {

	pendingRoles := make(map[string][]Member)

	// GraphQL query construction
	var qry struct {
		PendingProjectMemberChanges []struct {
			Project struct {
				Number graphql.String
			}
			Member struct {
				Username graphql.String
			}
			Action graphql.String
		} `graphql:"pendingProjectMemberChanges"`
	}

	if err := query(conf.AuthClientSecret, conf.AuthURL, conf.CoreAPIURL, &qry, nil); err != nil {
		log.Errorf("fail to query project pending roles: %s", err)
		return pendingRoles, err
	}

	for _, rc := range qry.PendingProjectMemberChanges {

		pid := string(rc.Project.Number)

		if _, ok := pendingRoles[pid]; !ok {
			pendingRoles[pid] = make([]Member, 0)
		}

		pendingRoles[pid] = append(pendingRoles[pid], Member{
			UserID: string(rc.Member.Username),
			Role:   action2role[string(rc.Action)],
		})
	}

	return pendingRoles, nil
}

// delProjectPendingRoles removes project pending role changes from the project database..
func delProjectPendingRoles(conf config.CoreAPIConfiguration, pendingRoles map[string][]Member) error {
	var mut struct {
		TotalRemoved graphql.Int `graphql:"removePendingProjectMemberChanges(changes: $changes)"`
	}

	// RemovePendingProjectMemberChangeInput is a JSON (Un)marshable data object that aligns to
	// the input type of the core api function: `RemovePendingProjectMemberChangesInput`.
	//
	// The JSON signature seems to be needed to allow the graphql library convert the object
	// into expected GraphQL input data. Furthermore, the type name should also be the same as
	// the input type defined by the core api, as the graphql library translates the type name
	// directly, which seems to be an undocumented behaviour/feature of the graphql library ...
	type RemovePendingProjectMemberChangeInput struct {
		Project graphql.ID     `json:"project"`
		Member  graphql.ID     `json:"member"`
		Action  graphql.String `json:"action"`
	}

	changes := make([]RemovePendingProjectMemberChangeInput, 0)
	cnt := 0
	for pid, members := range pendingRoles {
		for _, m := range members {
			changes = append(changes, RemovePendingProjectMemberChangeInput{
				Project: graphql.ID(pid),
				Member:  graphql.ID(m.UserID),
				Action:  graphql.String(role2action(m.Role)),
			})
			cnt++
		}
	}
	vars := map[string]interface{}{
		"changes": changes,
	}

	if err := mutate(conf.AuthClientSecret, conf.AuthURL, conf.CoreAPIURL, &mut, vars); err != nil {
		return err
	}

	if int(mut.TotalRemoved) != cnt {
		log.Warnf("unexpected number of changes deleted, expect %d removed %d", cnt, int(mut.TotalRemoved))
	}

	return nil
}

// role2action looks up a role in the `action2role` map and returns the corresponding
// action string.
func role2action(role string) string {
	action := ""

	for k, v := range action2role {
		if v == role {
			action = k
			break
		}
	}

	return action
}
