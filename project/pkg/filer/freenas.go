package filer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

const (
	// FREENAS_API_NS_DATASET is the API namespace for FreeNAS ZFS datasets.
	FREENAS_API_NS_DATASET string = "/pool/dataset"
)

// FreeNasConfig implements the `Config` interface and extends it with configurations
// that are specific to the FreeNas filer.
type FreeNasConfig struct {
	// ApiURL is the server URL of the OnTAP APIs.
	ApiURL string
	// ApiUser is the username for the basic authentication of the OnTAP API.
	ApiUser string
	// ApiPass is the password for the basic authentication of the OnTAP API.
	ApiPass string
	// ProjectRoot specifies the top-level NAS path in which projects are located.
	ProjectRoot string

	// ProjectUser specifies the system username for the owner of the project directory.
	ProjectUser string
	// ProjectGID specifies the system groupname for the owner of the project directory.
	ProjectGroup string

	// ZfsDatasetPrefix specifies the dataset prefix. It is usually started with the
	// zfs pool name followed by a top-level dataset name.  E.g. /zpool001/project.
	ZfsDatasetPrefix string
}

// GetApiURL returns the server URL of the OnTAP API.
func (c FreeNasConfig) GetApiURL() string {
	return strings.TrimSuffix(c.ApiURL, "/")
}

// GetApiUser returns the username for the API basic authentication.
func (c FreeNasConfig) GetApiUser() string { return c.ApiUser }

// GetApiPass returns the password for the API basic authentication.
func (c FreeNasConfig) GetApiPass() string { return c.ApiPass }

// GetProjectRoot returns the filesystem root path in which directories of projects are located.
func (c FreeNasConfig) GetProjectRoot() string { return c.ProjectRoot }

// FreeNas implements `Filer` for FreeNAS system.
type FreeNas struct {
	config FreeNasConfig
}

// CreateProject creates a new dataset on the FreeNAS system with the dataset size
// specified by `quotaGiB`.
func (filer FreeNas) CreateProject(projectID string, quotaGiB int) error {
	return nil
}

// CreateHome is not supported on FreeNAS and therefore it always returns an error.
func (filer FreeNas) CreateHome(username, groupname string, quotaGiB int) error {
	return fmt.Errorf("user home on FreeNAS is not supported")
}

// SetProjectQuota updates the size of the dataset for the specific dataset.
func (filer FreeNas) SetProjectQuota(projectID string, quotaGiB int) error {
	return nil
}

// SetHomeQuota is not supported on FreeNAS and therefore it always returns an error.
func (filer FreeNas) SetHomeQuota(username, groupname string, quotaGiB int) error {
	return fmt.Errorf("user home on FreeNAS is not supported")
}

// GetProjectQuotaInBytes returns the size of the dataset for a specific project in
// the unit of byte.
func (filer FreeNas) GetProjectQuotaInBytes(projectID string) (int64, error) {

	d, err := filer.getProjectDataset(projectID)

	if err != nil {
		return 0, fmt.Errorf("cannot get dataset for project %s: %s", projectID, err)
	}

	return d.RefQuota.Parsed, nil
}

// GetHomeQuotaInBytes is not supported on FreeNAS and therefore it always returns an error.
func (filer FreeNas) GetHomeQuotaInBytes(username, groupname string) (int64, error) {
	return 0, fmt.Errorf("user home on FreeNAS is not supported")
}

// getProjectDataset retrieves a structured dataset from the API.
func (filer FreeNas) getProjectDataset(projectID string) (*dataset, error) {

	c := newHTTPSClient(30*time.Second, true)

	// construct dataset id by prefixing the projectID with ZfsDatasetPrefix,
	// and url encode the whole id.
	id := url.PathEscape(strings.Join([]string{filer.config.ZfsDatasetPrefix, projectID}, "/"))
	log.Debugf("dataset id: %s", id)

	filer.config.GetApiURL()

	href := strings.Join([]string{
		filer.config.GetApiURL(),
		filepath.Join(FREENAS_API_NS_DATASET, "id", id),
	}, "")

	log.Debugf("href: %s", href)

	// create request
	req, err := http.NewRequest("GET", href, nil)
	if err != nil {
		return nil, err
	}

	// set request header for basic authentication
	req.SetBasicAuth(filer.config.GetApiUser(), filer.config.GetApiPass())
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	// expect status to be 200 (OK)
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("response not ok: %s (%d)", res.Status, res.StatusCode)
	}

	// read response body
	httpBodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// unmarshal response body to object structure
	d := dataset{}
	if err := json.Unmarshal(httpBodyBytes, &d); err != nil {
		return nil, err
	}

	return &d, nil
}

// dataset defines the JSON data structure of dataset retrieved from the API.
type dataset struct {
	ID          string     `json:"id"`
	Pool        string     `json:"pool"`
	Type        string     `json:"type"`
	SharedType  string     `json:"share_type"`
	Compression valueStr   `json:"compression"`
	RefQuota    valueInt64 `json:"refquota"`
	RecordSize  valueInt   `json:"recordsize"`
}

// valueStr defines general JSON structure of a string value retrieved from the API.
type valueStr struct {
	Value  string `json:"value,omitempty"`
	Parsed string `json:"parsed,omitempty"`
}

// valueInt64 defines general JSON structure of a int64 value retrieved from the API.
type valueInt64 struct {
	Value  string `json:"value,omitempty"`
	Parsed int64  `json:"parsed,omitempty"`
}

// valueInt defines general JSON structure of a int value retrieved from the API.
type valueInt struct {
	Value  string `json:"value,omitempty"`
	Parsed int    `json:"parsed,omitempty"`
}

// datasetUpdate defines the JSON data structure used to update a dataset.
type datasetUpdate struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	Sync            string `json:"sync"`
	Comments        string `json:"comments"`
	RefQuota        int64  `json:"refquota"`
	Compression     string `json:"compression"`
	Atime           string `json:"atime"`
	Exec            string `json:"exec"`
	Reservation     int    `json:"reservation"`
	RefReservation  int    `json:"refreservation"`
	Copies          int    `json:"copies"`
	Snapdir         string `json:"snapdir"`
	Deduplication   string `json:"deduplication"`
	ReadOnly        string `json:"readonly"`
	RecordSize      string `json:"recordsize"`
	CaseSensitivity string `json:"casesensitivity"`
	ShareType       string `json:"share_type"`
}
