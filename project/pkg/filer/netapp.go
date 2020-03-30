package filer

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

const (
	API_NS_SVMS        string = "/svm/svms"
	API_NS_JOBS        string = "/cluster/jobs"
	API_NS_VOLUMES     string = "/storage/volumes"
	API_NS_QTREES      string = "/storage/qtrees"
	API_NS_QUOTA_RULES string = "/storage/quota/rules"
)

type NetApp struct {
	ApiServerUrl string
	ApiUsername  string
	ApiPassword  string
}

func (filer NetApp) CreateProject(projectID string, quotaGiB int) error {
	return nil
}

func (filer NetApp) CreateHome(username, groupname string, quotaGiB int) error {
	return nil
}

func (filer NetApp) SetProjectQuota(projectID string, quotaGiB int) error {
	return nil
}

func (filer NetApp) SetHomeQuota(username, groupname string, quotaGiB int) error {
	return nil
}

// GetObjectByName retrives the named object from the given API namespace.
func (filer NetApp) GetObjectByName(name, nsAPI string, object interface{}) error {

	query := url.Values{}
	query.Set("name", name)

	return filer.GetObjectByQuery(query, nsAPI, object)

}

// GetObjectByQuery retrives the object from the given API namespace using a specific URL query.
func (filer NetApp) GetObjectByQuery(query url.Values, nsAPI string, object interface{}) error {
	c := newHTTPSClient()

	href := strings.Join([]string{filer.ApiServerUrl, "api", nsAPI}, "/")

	// create request
	req, err := http.NewRequest("GET", href, nil)
	if err != nil {
		return err
	}

	req.URL.RawQuery = query.Encode()

	// set request header for basic authentication
	req.SetBasicAuth(filer.ApiUsername, filer.ApiPassword)
	// NOTE: adding "Accept: application/json" to header can causes the API server
	//       to not returning "_links" attribute containing API href to the object.
	//       Therefore, it is not set here.
	//req.Header.Set("accept", "application/json")

	res, err := c.Do(req)
	if err != nil {
		return err
	}

	// expect status to be 200 (OK)
	if res.StatusCode != 200 {
		return fmt.Errorf("response not ok: %s (%d)", res.Status, res.StatusCode)
	}

	// read response body
	httpBodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	log.Debugf("%s", string(httpBodyBytes))

	// unmarshal response body to object structure
	if err := json.Unmarshal(httpBodyBytes, object); err != nil {
		return err
	}

	return nil
}

// internal utility functions
func newHTTPSClient() (client *http.Client) {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // FIXIT: don't ignore the bad server certificate.
	}

	client = &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}

	return
}

type Records struct {
	NumberOfRecords int      `json:"num_records"`
	Records         []Record `json:"records"`
}

type Record struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
	Link Link   `json:"_links"`
}

type Link struct {
	Self struct {
		Href string `json:"href"`
	} `json:"self"`
}

type Volume struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	Nas  Nas    `json:"nas"`
	Link Link   `json:"_links"`
}

type Nas struct {
	ExportPolicy struct {
		Name string `json:"name"`
	} `json:"export_policy"`
}
