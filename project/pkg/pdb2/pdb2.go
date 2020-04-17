// Package pdb2 implements connection to the Project Database 2.0,
// using the GraphQL interface.
package pdb2

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shurcooL/graphql"
	"golang.org/x/oauth2"

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

func init() {

	cfg := log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Info,
	}

	// initialize logger
	log.NewLogger(cfg, log.InstanceLogrusLogger)
}

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

// query is a generic function for GraphQL query.  It constructs a http client with a valid
// api token and performs graphql query to the project database's core api.
func query(authClientSecret string, qry interface{}, vars map[string]interface{}) error {
	// Perform query
	client, err := newClient(authClientSecret)
	if err != nil {
		return err
	}
	err = client.Query(context.Background(), qry, vars)
	if err != nil {
		return err
	}

	// Loop over qry and feed pendingRoles
	log.Debugf("qry result: %+v", qry)

	return nil
}

// newClient returns a GraphQL client with proper authentication via the
// authentication server.
func newClient(authClientSecret string) (*graphql.Client, error) {

	// retrieve API token
	token, err := getAuthToken(authClientSecret)
	if err != nil {
		return nil, err
	}

	// setup and return graphql client
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token.AccessToken},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	return graphql.NewClient(PDB_CORE_API_URL, httpClient), nil
}

// getAuthToken retrieves the OAuth client token from the authentication server,
// using the given clientSecret.
//
// Both client id and scope are hardcoded in this function.
func getAuthToken(clientSecret string) (*token, error) {

	clientID := "project-database-admin-script"
	clientScope := "project-database-core-api"

	// make a HTTP POST with FORM data to retrieve authentication token.
	href := strings.Join([]string{AUTH_SERVER_URL, "connect/token"}, "/")

	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("client_secret", clientSecret)
	v.Set("grant_type", "client_credentials")
	v.Set("scope", clientScope)

	c := newHTTPSClient(5*time.Second, false)
	res, err := c.PostForm(href, v)
	if err != nil {
		return nil, err
	}

	log.Debugf("status: %d message: %s", res.StatusCode, res.Status)

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	log.Debugf("response body: %s", string(bodyBytes))

	// unmarshal response body to Token struct
	t := token{}
	if err = json.Unmarshal(bodyBytes, &t); err != nil {
		return nil, err
	}

	log.Debugf("auth token: %s", t)

	return &t, nil
}

// newHTTPSClient initiate a HTTPS client.
func newHTTPSClient(timeout time.Duration, insecure bool) (client *http.Client) {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: timeout,
		}).DialContext,
		TLSHandshakeTimeout: timeout,
	}

	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	client = &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	return
}

// token holds the data structure of the Core API access token.
type token struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}
