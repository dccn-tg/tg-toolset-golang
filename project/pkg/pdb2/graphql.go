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
	"sync"
	"time"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/shurcooL/graphql"
	"golang.org/x/oauth2"
)

var (
	// apiToken is the global reusable api token before it becomes
	// invalid.
	apiToken token
)

// token holds the data structure of the Core API access token.
type token struct {
	AccessToken string
	TokenType   string
	validUntil  time.Time
	mux         sync.Mutex
}

// query is a generic function for performing the GraphQL query.
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

// newClient returns a GraphQL client with proper and valid authentication.
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

// getAuthToken retrieves a valid OAuth authentication token.
//
// If the global `apiToken` is still valid, it is returned rightaway. Otherwise,
// this function renews the `apiToken` using the given `clientSecret` before
// returning the `apiToken`.
//
// The `clientID` and `clientScope` used for renewing the `apiToken` are hardcoded
// in this function.
func getAuthToken(clientSecret string) (*token, error) {

	// lock the apiToken for eventual manipulation of it.
	apiToken.mux.Lock()

	// make sure apiToken is unlocked for concurrent processes to use it.
	defer apiToken.mux.Unlock()

	// return the global reusable token if it is still valid in 5 minutes.
	if time.Now().Add(5 * time.Minute).Before(apiToken.validUntil) {
		return &apiToken, nil
	}

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
	var t struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err = json.Unmarshal(bodyBytes, &t); err != nil {
		return nil, err
	}

	log.Debugf("auth token: %s", t)

	apiToken.AccessToken = t.AccessToken
	apiToken.TokenType = t.TokenType
	apiToken.validUntil = time.Now().Add(time.Second * time.Duration(t.ExpiresIn))

	return &apiToken, nil
}

// newHTTPSClient initiate a new HTTPS client.
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
