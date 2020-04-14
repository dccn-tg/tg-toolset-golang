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
)

var (
	// PDB_CORE_API_URL is the PDB2 core api server URL.
	PDB_CORE_API_URL string

	// AUTH_SERVER_URL is the authentication server URL.
	AUTH_SERVER_URL string
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

// ProjectPendingRole is the data structure of a pending role setting on a project.
type ProjectPendingRole struct {
	ProjectID string `json:"projectID"`
	Username  string `json:"username"`
	Role      string `json:"role"`
}

// GetProjectPendingRoles retrieves a list of project pending roles from the project
// database.
func GetProjectPendingRoles(authClientSecret string) ([]ProjectPendingRole, error) {

	pendingRoles := make([]ProjectPendingRole, 0)

	// GraphQL query construction
	var qry struct {
		PendingProjectMemberChanges struct {
			Project struct {
				Number graphql.String
			}
			Member struct {
				Username graphql.String
			}
			Action graphql.String
		} `graphql:"pendingProjectMemberChanges"`
	}

	// Perform query
	client, err := newClient(authClientSecret)
	if err != nil {
		return pendingRoles, err
	}
	err = client.Query(context.Background(), &qry, nil)
	if err != nil {
		return pendingRoles, err
	}

	// Loop over qry and feed pendingRoles

	return pendingRoles, nil
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

	return graphql.NewClient("https://example.com/graphql", httpClient), nil
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
