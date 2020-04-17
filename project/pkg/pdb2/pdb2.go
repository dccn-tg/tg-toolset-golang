package pdb2

import (
	"github.com/shurcooL/graphql"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"
)

var (
	// PDB_CORE_API_URL is the PDB2 core api server URL.
	PDB_CORE_API_URL string

	// AUTH_SERVER_URL is the authentication server URL.
	AUTH_SERVER_URL string

	// action2role maps the pending role action string of the Core API
	// to the string representation of the `acl.Role`, and can be used
	// directly for the API of the filer-gateway:
	//
	// https://github.com/Donders-Institute/filer-gateway
	action2role map[string]string = map[string]string{
		"SetToManager":     acl.Manager.String(),
		"SetToContributor": acl.Contributor.String(),
		"SetToViewer":      acl.Viewer.String(),
		"Unset":            "none",
	}
)

// member is the data structure of a pending role setting on a project.
type member struct {
	UserID string `json:"userID"`
	Role   string `json:"role"`
}

// storage is the data structure for project quota information.
type storage struct {
	QuotaGb int    `json:"quotaGb"`
	System  string `json:"system"`
}

// DataProjectProvision defines the data structure for project provisioning with
// given storage quota and data-access roles.
type DataProjectProvision struct {
	ProjectID string   `json:"projectID"`
	Members   []member `json:"members"`
	Storage   storage  `json:"storage"`
}

// DataProjectUpdate defines the data structure for project update with given
// storage quota and data-access roles.
type DataProjectUpdate struct {
	Members []member `json:"members"`
	Storage storage  `json:"storage"`
}

// GetProjectPendingActions performs queries on project pending roles and project storage
// resource, and combines the results into data structure that can be directly used for
// updating project storage resources and member roles via the filer-gateway API:
// https://github.com/Donders-Institute/filer-gateway
func GetProjectPendingActions(authClientSecret string) (map[string]DataProjectUpdate, error) {
	actions := make(map[string]DataProjectUpdate)

	pendingRoles, err := getProjectPendingRoles(authClientSecret)
	if err != nil {
		return actions, err
	}

	for pid, members := range pendingRoles {
		stor, err := getProjectStorageResource(authClientSecret, pid)
		if err != nil {
			log.Errorf("%s", err)
		}
		actions[pid] = DataProjectUpdate{
			Members: members,
			Storage: *stor,
		}
	}

	return actions, nil
}

// getProjectStorageResource retrieves the storage resource information of a given project.
func getProjectStorageResource(authClientSecret, projectID string) (*storage, error) {
	var stor storage

	// GraphQL query construction
	var qry struct {
		Project struct {
			QuotaGb graphql.Int
		} `graphql:"project(number: $id)"`
	}

	vars := map[string]interface{}{
		"id": graphql.ID(projectID),
	}

	if err := query(authClientSecret, &qry, vars); err != nil {
		log.Errorf("fail to query project quota: %s", err)
		return nil, err
	}

	// TODO: do not hardcode system to "netapp".
	stor = storage{
		QuotaGb: int(qry.Project.QuotaGb),
		System:  "netapp",
	}

	return &stor, nil
}

// getProjectPendingRoles retrieves a list of project pending roles from the project
// database.
func getProjectPendingRoles(authClientSecret string) (map[string][]member, error) {

	pendingRoles := make(map[string][]member)

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

	if err := query(authClientSecret, &qry, nil); err != nil {
		log.Errorf("fail to query project pending roles: %s", err)
		return pendingRoles, err
	}

	for _, rc := range qry.PendingProjectMemberChanges {

		pid := string(rc.Project.Number)

		if _, ok := pendingRoles[pid]; !ok {
			pendingRoles[pid] = make([]member, 0)
		}

		pendingRoles[pid] = append(pendingRoles[pid], member{
			UserID: string(rc.Member.Username),
			Role:   action2role[string(rc.Action)],
		})
	}

	return pendingRoles, nil
}
