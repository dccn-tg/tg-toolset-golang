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

	log "github.com/dccn-tg/tg-toolset-golang/pkg/logger"
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

// oauth2HttpClient returns a HTTP client with a valid OAuth2 access token.
func oauth2HttpClient(authClientID, authClientSecret, authURL string) (*http.Client, error) {

	// retrieve API token
	token, err := getAuthToken(authClientID, authClientSecret, authURL)
	if err != nil {
		return nil, err
	}

	// setup and return graphql client
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token.AccessToken},
	)

	return oauth2.NewClient(context.Background(), src), nil
}

// getAuthToken returns a valid OAuth authentication token.
//
// If the global `apiToken` is still valid, it is returned rightaway. Otherwise,
// this function renews the `apiToken` using the given `clientSecret` before
// returning the `apiToken`.
func getAuthToken(clientID, clientSecret, authURL string) (*token, error) {

	// lock the apiToken for eventual manipulation of it.
	apiToken.mux.Lock()

	// make sure apiToken is unlocked for concurrent processes to use it.
	defer apiToken.mux.Unlock()

	// return the global reusable token if it is still valid in 5 minutes.
	if time.Now().Add(5 * time.Minute).Before(apiToken.validUntil) {
		return &apiToken, nil
	}

	// make a HTTP POST with FORM data to retrieve authentication token.
	href := strings.Join([]string{authURL, "connect/token"}, "/")

	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("client_secret", clientSecret)
	v.Set("grant_type", "client_credentials")
	v.Set("scope", "urn:dccn:pdb:core-api:query")

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

// newHTTPSClient initiates a new HTTPS client.
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
