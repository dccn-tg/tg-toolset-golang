package filer

import (
	"bytes"
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
	// API_NS_SVMS is the API namespace for OnTAP SVM items.
	API_NS_SVMS string = "/svm/svms"
	// API_NS_JOBS is the API namespace for OnTAP cluster job items.
	API_NS_JOBS string = "/cluster/jobs"
	// API_NS_VOLUMES is the API namespace for OnTAP volume items.
	API_NS_VOLUMES string = "/storage/volumes"
	// API_NS_AGGREGATES is the API namespace for OnTAP aggregate items.
	API_NS_AGGREGATES string = "/storage/aggregates"
	// API_NS_QTREES is the API namespace for OnTAP qtree items.
	API_NS_QTREES string = "/storage/qtrees"
	// API_NS_QUOTA_RULES is the API namespace for OnTAP quota rule items.
	API_NS_QUOTA_RULES string = "/storage/quota/rules"
)

// NetAppConfig implements the `Config` interface and extends it with configurations
// that are specific to the NetApp filer.
type NetAppConfig struct {
	// APIServerURL is the server URL of the OnTAP APIs.
	APIServerURL string
	// APIUsername is the username for the basic authentication of the OnTAP API.
	APIUsername string
	// APIPassword is the password for the basic authentication of the OnTAP API.
	APIPassword string
	// ProjectRoot specifies the top-level NAS path in which projects are located.
	ProjectRoot string

	// ProjectMode specifies how the project space is allocated. Valid modes are
	// "volume" and "qtree".
	ProjectMode string
	// Vserver specifies the name of OnTAP SVM on which the filer APIs will perform.
	Vserver string
	// ProjectUID specifies the system UID of user `project`
	ProjectUID int
	// ProjectGID specifies the system GID of group `project_g`
	ProjectGID int
}

// GetApiURL returns the server URL of the OnTAP API.
func (c NetAppConfig) GetApiURL() string { return c.APIServerURL }

// GetApiUser returns the username for the API basic authentication.
func (c NetAppConfig) GetApiUser() string { return c.APIUsername }

// GetApiPass returns the password for the API basic authentication.
func (c NetAppConfig) GetApiPass() string { return c.APIPassword }

// GetProjectRoot returns the filesystem root path in which directories of projects are located.
func (c NetAppConfig) GetProjectRoot() string { return c.ProjectRoot }

// NetApp implements Filer interface for NetApp OnTAP cluster.
type NetApp struct {
	config NetAppConfig
}

// volName converts project identifier to the OnTAP volume name.
//
// e.g. 3010000.01 -> project_3010000_01
func (filer NetApp) volName(projectID string) string {
	return strings.Join([]string{
		"project",
		strings.ReplaceAll(projectID, ".", "_"),
	}, "_")
}

// CreateProject provisions a project space on the filer with the given quota.
func (filer NetApp) CreateProject(projectID string, quotaGiB int) error {

	switch filer.config.ProjectMode {
	case "volume":
		// check if volume with the same name doee not exist.
		qry := url.Values{}
		qry.Set("name", filer.volName(projectID))
		records, err := filer.getRecordsByQuery(qry, API_NS_VOLUMES)
		if err != nil {
			return fmt.Errorf("fail to check volume %s: %s", projectID, err)
		}
		if len(records) != 0 {
			return fmt.Errorf("project volume already exists: %s", projectID)
		}

		// determine which aggregate should be used for creating the new volume.
		quota := int64(quotaGiB << 30)
		svm := SVM{}
		if err := filer.getObjectByName(filer.config.Vserver, API_NS_SVMS, &svm); err != nil {
			return fmt.Errorf("fail to get SVM %s: %s", filer.config.Vserver, err)
		}
		avail := int64(0)

		var theAggr *Aggregate
		for _, record := range svm.Aggregates {
			aggr := Aggregate{}
			href := strings.Join([]string{
				"/api",
				API_NS_AGGREGATES,
				record.UUID,
			}, "/")
			if err := filer.getObjectByHref(href, &aggr); err != nil {
				log.Errorf("ignore aggregate %s: %s", record.Name, err)
			}
			if aggr.State == "online" && aggr.Space.BlockStorage.Available > avail && aggr.Space.BlockStorage.Available > quota {
				theAggr = &aggr
			}
		}

		if theAggr == nil {
			return fmt.Errorf("cannot find aggregate for creating volume")
		}
		log.Debugf("selected aggreate for project volume: %+v", *theAggr)

		// create project volume with given quota.
		vol := Volume{
			Name: filer.volName(projectID),
			Aggregates: []Record{
				Record{Name: theAggr.Name},
			},
			Size:  quota,
			Svm:   Record{Name: filer.config.Vserver},
			State: "online",
			Style: "flexvol",
			Type:  "rw",
			Nas: Nas{
				UID:             filer.config.ProjectUID,
				GID:             filer.config.ProjectGID,
				Path:            filepath.Join(filer.config.GetProjectRoot(), projectID),
				SecurityStyle:   "unix",
				UnixPermissions: "0750",
				ExportPolicy:    ExportPolicy{Name: "dccn-projects"},
			},
			QoS: &QoS{
				Policy: QoSPolicy{MaxIOPS: 6000},
			},
			Autosize: &Autosize{Mode: "off"},
		}

		// blocking operation to create the volume.
		if err := filer.createObject(&vol, API_NS_VOLUMES); err != nil {
			return err
		}

	case "qtree":
		return fmt.Errorf("not implemented yet: %s", filer.config.ProjectMode)

	default:
		return fmt.Errorf("unsupported project mode: %s", filer.config.ProjectMode)
	}

	return nil
}

// CreateHome creates a home directory as qtree `username` under the volume `groupname`,
// and assigned the given `quotaGiB` to the qtree.
func (filer NetApp) CreateHome(username, groupname string, quotaGiB int) error {

	// check if volume "groupname" exists.
	qry := url.Values{}
	qry.Set("name", groupname)
	volRecords, err := filer.getRecordsByQuery(qry, API_NS_VOLUMES)
	if err != nil {
		return fmt.Errorf("fail to check volume %s: %s", groupname, err)
	}
	if len(volRecords) == 0 {
		return fmt.Errorf("volume doesn't exit: %s", groupname)
	}

	// check if qtree with "username" already exists.
	qry.Set("name", username)
	qry.Set("volume.name", groupname)
	records, err := filer.getRecordsByQuery(qry, API_NS_QTREES)
	if err != nil {
		return fmt.Errorf("fail to check qtree %s: %s", username, err)
	}
	if len(records) != 0 {
		return fmt.Errorf("qtree already exists: %s", username)
	}

	// create qtree within the volume.
	qtree := QTree{
		Name:            username,
		SVM:             Record{Name: filer.config.Vserver},
		Volume:          Record{Name: groupname},
		SecurityStyle:   "unix",
		UnixPermissions: "0700",
		ExportPolicy:    ExportPolicy{Name: "dccn-home-nfs-vpn"},
	}

	qrule := QuotaRule{
		SVM:    Record{Name: filer.config.Vserver},
		Volume: Record{Name: groupname},
		QTree:  &Record{Name: username},
		Type:   "tree",
		Space:  &QuotaLimit{HardLimit: int64(quotaGiB << 30)},
	}

	// blocking operation to create the qtree.
	if err := filer.createObject(&qtree, API_NS_QTREES); err != nil {
		return err
	}

	// switch off volume quota
	// otherwise we cannot create new rule; perhaps it is because we don't have default rule on the volume.
	if err := filer.patchObject(volRecords[0], []byte(`{"quota":{"enabled":false}}`)); err != nil {
		return err
	}

	// ensure the volume quota will be switched on before this function is returned.
	defer func() {
		if err := filer.patchObject(volRecords[0], []byte(`{"quota":{"enabled":true}}`)); err != nil {
			log.Errorf("cannot turn on quota for volume %s: %s", groupname, err)
		}
	}()

	// create quota rule for the newly created qtree.
	if err := filer.createObject(&qrule, API_NS_QUOTA_RULES); err != nil {
		return err
	}

	return nil
}

// SetProjectQuota updates the quota of a project space.
func (filer NetApp) SetProjectQuota(projectID string, quotaGiB int) error {
	switch filer.config.ProjectMode {
	case "volume":
		// check if volume with the same name already exists.
		qry := url.Values{}
		qry.Set("name", filer.volName(projectID))
		records, err := filer.getRecordsByQuery(qry, API_NS_VOLUMES)
		if err != nil {
			return fmt.Errorf("fail to check volume %s: %s", projectID, err)
		}
		if len(records) != 1 {
			return fmt.Errorf("project volume doesn't exist: %s", projectID)
		}

		// resize the volume to the given quota.
		data := []byte(fmt.Sprintf(`{"name":"%s", "size":%d}`, filer.volName(projectID), quotaGiB<<30))

		if err := filer.patchObject(records[0], data); err != nil {
			return err
		}

	case "qtree":
		return fmt.Errorf("unsupported project mode: %s", filer.config.ProjectMode)

	default:
		return fmt.Errorf("unsupported project mode: %s", filer.config.ProjectMode)
	}

	return nil
}

// SetHomeQuota updates the quota of a home directory.
func (filer NetApp) SetHomeQuota(username, groupname string, quotaGiB int) error {

	// check if the volume exists
	qry := url.Values{}
	qry.Set("name", groupname)
	volRecords, err := filer.getRecordsByQuery(qry, API_NS_VOLUMES)
	if err != nil {
		return fmt.Errorf("fail to check volume %s: %s", groupname, err)
	}
	if len(volRecords) == 0 {
		return fmt.Errorf("volume doesn't exit: %s", groupname)
	}

	// check if the quota rule exists
	qry = url.Values{}
	qry.Set("volume.name", groupname)
	qry.Set("qtree.name", username)
	records, err := filer.getRecordsByQuery(qry, API_NS_QUOTA_RULES)
	if err != nil {
		return fmt.Errorf("fail to check quota rule for volume %s qtree %s: %s", groupname, username, err)
	}
	if len(records) != 1 {
		return fmt.Errorf("quota rule for volume %s qtree %s doesn't exist", groupname, username)
	}

	// switch off volume quota
	// otherwise we cannot create new rule; perhaps it is because we don't have default rule on the volume.
	if err := filer.patchObject(volRecords[0], []byte(`{"quota":{"enabled":false}}`)); err != nil {
		return err
	}

	// ensure the volume quota will be switched on before this function is returned.
	defer func() {
		if err := filer.patchObject(volRecords[0], []byte(`{"quota":{"enabled":true}}`)); err != nil {
			log.Errorf("cannot turn on quota for volume %s: %s", groupname, err)
		}
	}()

	// update corresponding quota rule for the qtree
	data := []byte(fmt.Sprintf(`{"space":{"hard_limit":%d}}`, quotaGiB<<30))

	if err := filer.patchObject(records[0], data); err != nil {
		return err
	}

	return nil
}

// getObjectByName retrives the named object from the given API namespace.
func (filer NetApp) getObjectByName(name, nsAPI string, object interface{}) error {

	query := url.Values{}
	query.Set("name", name)

	records, err := filer.getRecordsByQuery(query, nsAPI)
	if err != nil {
		return err
	}

	if len(records) != 1 {
		return fmt.Errorf("more than 1 object found: %d", len(records))
	}

	if err := filer.getObjectByHref(records[0].Link.Self.Href, object); err != nil {
		return err
	}

	return nil
}

// getRecordsByQuery retrives the object from the given API namespace using a specific URL query.
func (filer NetApp) getRecordsByQuery(query url.Values, nsAPI string) ([]Record, error) {

	records := make([]Record, 0)

	c := newHTTPSClient(30*time.Second, true)

	href := strings.Join([]string{filer.config.GetApiURL(), "api", nsAPI}, "/")

	// create request
	req, err := http.NewRequest("GET", href, nil)
	if err != nil {
		return records, err
	}

	req.URL.RawQuery = query.Encode()

	// set request header for basic authentication
	req.SetBasicAuth(filer.config.GetApiUser(), filer.config.GetApiPass())
	// NOTE: adding "Accept: application/json" to header can causes the API server
	//       to not returning "_links" attribute containing API href to the object.
	//       Therefore, it is not set here.
	//req.Header.Set("accept", "application/json")

	res, err := c.Do(req)
	if err != nil {
		return records, err
	}

	// expect status to be 200 (OK)
	if res.StatusCode != 200 {
		return records, fmt.Errorf("response not ok: %s (%d)", res.Status, res.StatusCode)
	}

	// read response body
	httpBodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return records, err
	}

	// unmarshal response body to object structure
	rec := Records{}
	if err := json.Unmarshal(httpBodyBytes, &rec); err != nil {
		return records, err
	}

	return rec.Records, nil
}

// getObjectByHref retrives the object from the given API namespace using a specific URL query.
func (filer NetApp) getObjectByHref(href string, object interface{}) error {

	c := newHTTPSClient(10*time.Second, true)

	// create request
	req, err := http.NewRequest("GET", strings.Join([]string{filer.config.GetApiURL(), href}, "/"), nil)
	if err != nil {
		return err
	}

	// set request header for basic authentication
	req.SetBasicAuth(filer.config.GetApiUser(), filer.config.GetApiPass())
	// NOTE: adding "Accept: application/json" to header can causes the API server
	//       to not returning "_links" attribute containing API href to the object.
	//       Therefore, it is not set here.
	//req.Header.Set("accept", "application/json")

	res, err := c.Do(req)

	// expect status to be 200 (OK)
	if res.StatusCode != 200 {
		return fmt.Errorf("response not ok: %s (%d)", res.Status, res.StatusCode)
	}

	// read response body
	httpBodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	// unmarshal response body to object structure
	if err := json.Unmarshal(httpBodyBytes, object); err != nil {
		return err
	}

	return nil
}

// createObject creates given object under the specified API namespace.
func (filer NetApp) createObject(object interface{}, nsAPI string) error {
	c := newHTTPSClient(10*time.Second, true)

	href := strings.Join([]string{filer.config.GetApiURL(), "api", nsAPI}, "/")

	data, err := json.Marshal(object)

	if err != nil {
		return fmt.Errorf("fail to convert to json data: %+v, %s", object, err)
	}

	log.Debugf("object creation input: %s", string(data))

	// create request
	req, err := http.NewRequest("POST", href, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	// set request header for basic authentication
	req.SetBasicAuth(filer.config.GetApiUser(), filer.config.GetApiPass())
	req.Header.Set("content-type", "application/json")

	res, err := c.Do(req)
	if err != nil {
		return err
	}

	// expect status to be 202 (Accepted)
	if res.StatusCode != 202 {
		// try to get the error code returned as the body
		var apiErr APIError
		if httpBodyBytes, err := ioutil.ReadAll(res.Body); err == nil {
			json.Unmarshal(httpBodyBytes, &apiErr)
		}
		return fmt.Errorf("response not ok: %s (%d), error: %+v", res.Status, res.StatusCode, apiErr)
	}

	// read response body as accepted job
	httpBodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("cannot read response body: %s", err)
	}

	job := APIJob{}
	// unmarshal response body to object structure
	if err := json.Unmarshal(httpBodyBytes, &job); err != nil {
		return fmt.Errorf("cannot get job id: %s", err)
	}

	log.Debugf("job data: %+v", job)

	if err := filer.waitJob(&job); err != nil {
		return err
	}

	if job.Job.State != "success" {
		return fmt.Errorf("API job failed: %s", job.Job.Message)
	}

	return nil
}

// patchObject patches given object `Record` with provided setting specified by `data`.
func (filer NetApp) patchObject(object Record, data []byte) error {

	c := newHTTPSClient(10*time.Second, true)

	href := strings.Join([]string{filer.config.GetApiURL(), object.Link.Self.Href}, "/")

	// create request
	req, err := http.NewRequest("PATCH", href, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	// set request header for basic authentication
	req.SetBasicAuth(filer.config.GetApiUser(), filer.config.GetApiPass())
	req.Header.Set("content-type", "application/json")

	res, err := c.Do(req)
	if err != nil {
		return err
	}

	// expect status to be 202 (Accepted)
	if res.StatusCode != 202 {
		// try to get the error code returned as the body
		var apiErr APIError
		if httpBodyBytes, err := ioutil.ReadAll(res.Body); err == nil {
			json.Unmarshal(httpBodyBytes, &apiErr)
		}
		return fmt.Errorf("response not ok: %s (%d), error: %+v", res.Status, res.StatusCode, apiErr)
	}

	// read response body as accepted job
	httpBodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("cannot read response body: %s", err)
	}

	job := APIJob{}
	// unmarshal response body to object structure
	if err := json.Unmarshal(httpBodyBytes, &job); err != nil {
		return fmt.Errorf("cannot get job id: %s", err)
	}

	log.Debugf("job data: %+v", job)

	if err := filer.waitJob(&job); err != nil {
		return err
	}

	if job.Job.State != "success" {
		return fmt.Errorf("API job failed: %s", job.Job.Message)
	}

	return nil
}

// waitJob polls the status of the api job unti it if finished; and reports the job's final state.
func (filer NetApp) waitJob(job *APIJob) error {

	var err error

	href := job.Job.Link.Self.Href

waitLoop:
	for {
		if e := filer.getObjectByHref(href, &(job.Job)); err != nil {
			err = fmt.Errorf("cannot poll job %s: %s", job.Job.UUID, e)
			break
		}

		log.Debugf("job status: %s", job.Job.State)

		switch job.Job.State {
		case "success":
			break waitLoop
		case "failure":
			break waitLoop
		default:
			time.Sleep(3 * time.Second)
			continue waitLoop
		}
	}

	return err
}

// APIJob of the API request.
type APIJob struct {
	Job Job `json:"job"`
}

// Job detail of the API request.
type Job struct {
	Link    *Link  `json:"_links"`
	UUID    string `json:"uuid"`
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`
}

// APIError of the API request.
type APIError struct {
	Error struct {
		Target    string `json:"target"`
		Arguments struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"arguments"`
	} `json:"error"`
}

// Records of the items within an API namespace.
type Records struct {
	NumberOfRecords int      `json:"num_records"`
	Records         []Record `json:"records"`
}

// Record of an item within an API namespace.
type Record struct {
	UUID string `json:"uuid,omitempty"`
	Name string `json:"name,omitempty"`
	Link *Link  `json:"_links,omitempty"`
}

// Link of an item for retriving the detail.
type Link struct {
	Self struct {
		Href string `json:"href"`
	} `json:"self"`
}

// Volume of OnTAP.
type Volume struct {
	UUID       string    `json:"uuid,omitempty"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	State      string    `json:"state"`
	Size       int64     `json:"size"`
	Style      string    `json:"style"`
	Space      *Space    `json:"space,omitempty"`
	Svm        Record    `json:"svm"`
	Aggregates []Record  `json:"aggregates"`
	Nas        Nas       `json:"nas"`
	QoS        *QoS      `json:"qos,omitempty"`
	Autosize   *Autosize `json:"autosize,omitempty"`
	Link       *Link     `json:"_links,omitempty"`
}

// QoS contains a Qolity-of-Service policy.
type QoS struct {
	Policy QoSPolicy `json:"policy"`
}

// QoSPolicy defines the data structure of the QoS policy.
type QoSPolicy struct {
	MaxIOPS int    `json:"max_throughput_iops,omitempty"`
	MaxMBPS int    `json:"max_throughput_mbps,omitempty"`
	UUID    string `json:"uuid,omitempty"`
	Name    string `json:"name,omitempty"`
}

// Autosize defines the volume autosizing mode
type Autosize struct {
	Mode string `json:"mode"`
}

// Nas related attribute of OnTAP.
type Nas struct {
	Path            string       `json:"path,omitempty"`
	UID             int          `json:"uid,omitempty"`
	GID             int          `json:"gid,omitempty"`
	SecurityStyle   string       `json:"security_style,omitempty"`
	UnixPermissions string       `json:"unix_permissions,omitempty"`
	ExportPolicy    ExportPolicy `json:"export_policy,omitempty"`
}

// ExportPolicy defines the export policy for a volume or a qtree.
type ExportPolicy struct {
	Name string `json:"name"`
}

// Space information of a OnTAP volume.
type Space struct {
	Size      int64 `json:"size"`
	Available int64 `json:"available"`
	Used      int64 `json:"used"`
}

// SVM of OnTAP
type SVM struct {
	UUID       string   `json:"uuid"`
	Name       string   `json:"name"`
	State      string   `json:"state"`
	Aggregates []Record `json:"aggregates"`
}

// Aggregate of OnTAP
type Aggregate struct {
	UUID  string `json:"uuid"`
	Name  string `json:"name"`
	State string `json:"state"`
	Space struct {
		BlockStorage Space `json:"block_storage"`
	} `json:"space"`
}

// QTree of OnTAP
type QTree struct {
	ID              string       `json:"id,omitempty"`
	Name            string       `json:"name"`
	Path            string       `json:"path,omitempty"`
	SVM             Record       `json:"svm"`
	Volume          Record       `json:"volume"`
	ExportPolicy    ExportPolicy `json:"export_policy"`
	SecurityStyle   string       `json:"security_style"`
	UnixPermissions string       `json:"unix_permissions"`
	Link            *Link        `json:"_links,omitempty"`
}

// QuotaRule of OnTAP
type QuotaRule struct {
	SVM    Record      `json:"svm"`
	Volume Record      `json:"volume"`
	QTree  *Record     `json:"qtree,omitempty"`
	Users  *Record     `json:"users,omitempty"`
	Group  *Record     `json:"group,omitempty"`
	Type   string      `json:"type"`
	Space  *QuotaLimit `json:"space,omitempty"`
	Files  *QuotaLimit `json:"files,omitempty"`
}

// QuotaLimit defines the quota limitation.
type QuotaLimit struct {
	HardLimit int64 `json:"hard_limit,omitempty"`
	SoftLimit int64 `json:"soft_limit,omitempty"`
}
