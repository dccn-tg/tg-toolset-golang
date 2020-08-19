// Package filergateway provides client interfaces of the filer-gateway.
package filergateway

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"

	fgwcli "github.com/Donders-Institute/filer-gateway/pkg/swagger/client/client"
	fgwops "github.com/Donders-Institute/filer-gateway/pkg/swagger/client/client/operations"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

// NewClient returns a new `filerGateway` instance with settings
// given by the `config` file.
func NewClient(config config.Configuration) (Client, error) {
	return Client{
		apiKey:  config.FilerGateway.APIKey,
		apiURL:  config.FilerGateway.APIURL,
		apiUser: config.FilerGateway.APIUser,
		apiPass: config.FilerGateway.APIPass,
	}, nil
}

// ServiceTask is the data structure representing the asynchronous filer-gateway task.
type ServiceTask struct {
	TaskID     string            `json:"taskID"`
	TaskStatus ServiceTaskStatus `json:"taskStatus"`
}

// isCompleted checks whether the task is at one of the end states: `failed`, `succeeded`, `canceled`.
func (t *ServiceTask) isCompleted() bool {
	switch t.TaskStatus.Status {
	case "failed", "succeeded", "canceled":
		return true
	default:
		return false
	}
}

// ServiceTaskStatus is the data structure representing the status detail of a filer-gateway task.
type ServiceTaskStatus struct {
	Error  string `json:"error"`
	Result string `json:"result"`
	Status string `json:"status"`
}

// ServiceError is the data structure representing the filer-gateway internal error messages
// for API error code 400 and 500.
type ServiceError struct {
	ErrorMessage string `json:"errorMessage"`
	ExitCode     int    `json:"exitCode,omitempty"`
}

// Error implements the `error` interface.
func (e *ServiceError) Error() string {
	return fmt.Sprintf("filer-gateway error: %s (ec:%d)", e.ErrorMessage, e.ExitCode)
}

// Client implements client interfaces of the FilerGateway.
type Client struct {
	apiKey  string
	apiURL  string
	apiUser string
	apiPass string
}

// GetProject returns storage information of a project retrieved from the filer-gateway.
func (f *Client) GetProject(projectID string) (*pdb.DataProjectProvision, error) {

	url, err := url.Parse(f.apiURL)
	if err != nil {
		return nil, err
	}

	c := fgwcli.NewHTTPClientWithConfig(
		nil,
		&fgwcli.TransportConfig{
			Host:     url.Host,
			BasePath: url.Path,
			Schemes:  []string{url.Scheme},
		},
	)

	// request data with timeout
	req := fgwops.NewGetProjectsIDParamsWithTimeout(10 * time.Second)
	req.ID = projectID

	res, err := c.Operations.GetProjectsID(req)
	if err != nil {
		return nil, err
	}

	// serialize response payload to byte data.
	data, err := json.Marshal(res.Payload)
	if err != nil {
		return nil, err
	}
	log.Debugf("response data: %s", data)

	// serialize byte data into pdb.DataProjectProvision
	var info pdb.DataProjectProvision
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// UpdateProject updates or creates filer storage with information given by the `data`.
func (f *Client) UpdateProject(projectID string, data *pdb.DataProjectUpdate) (*ServiceTask, error) {

	// perform request
	dpatch, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// new http client with timeout of 5 seconds
	c := newHTTPSClient(time.Second*5, false)

	// create request
	req, err := http.NewRequest("PATCH", strings.Join([]string{f.apiURL, "projects", projectID}, "/"), bytes.NewBuffer(dpatch))
	if err != nil {
		return nil, err
	}

	// set request headers: content-type, X-API-KEY
	req.Header.Set("content-type", "application/json")
	req.Header.Set("X-API-KEY", f.apiKey)

	// set basic auth
	req.SetBasicAuth(f.apiUser, f.apiPass)

	// perform request to te filer-gateway
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	// handle returned status code
	httpBodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	switch res.StatusCode {
	case 200:
		var task ServiceTask
		if err := json.Unmarshal(httpBodyBytes, &task); err != nil {
			return nil, err
		}
		return &task, nil
	case 400, 500:
		var serr ServiceError
		if err := json.Unmarshal(httpBodyBytes, &serr); err != nil {
			return nil, err
		}
		return nil, &serr
	default:
		return nil, fmt.Errorf("%s", httpBodyBytes)
	}
}

// SyncUpdateProject performs project update on the filer and wait until the asynchronous task to be
// finished.
func (f *Client) SyncUpdateProject(projectID string, data *pdb.DataProjectUpdate, timeGapPoll time.Duration) (*ServiceTask, error) {

	task, err := f.UpdateProject(projectID, data)
	if err != nil {
		return task, err
	}

	for {
		// sleep for given `timeGapPoll` and make the next poll on task status
		time.Sleep(timeGapPoll)
		task, err = f.GetTaskStatus(task.TaskID, "project")

		// break the loop if task status polling is failed
		if err != nil {
			break
		}

		// break the loop if task has reached its final state
		if task.isCompleted() {
			break
		}
	}

	// analyze task result and return error if the task is not succeeded.
	if task.TaskStatus.Status != "succeeded" {
		return task, fmt.Errorf("task %s not succeeded", task.TaskID)
	}

	// return the final version of task and err
	return task, err
}

// GetTaskStatus performs a query to retrieve the status of a filer-gateway task.
func (f *Client) GetTaskStatus(taskID string, taskType string) (*ServiceTask, error) {

	// new http client with timeout of 5 seconds
	c := newHTTPSClient(time.Second*5, false)

	// create request
	req, err := http.NewRequest("GET", strings.Join([]string{f.apiURL, "tasks", taskType, taskID}, "/"), nil)
	if err != nil {
		return nil, err
	}

	// set request headers: content-type, X-API-KEY
	req.Header.Set("content-type", "application/json")
	req.Header.Set("X-API-KEY", f.apiKey)

	// perform request to te filer-gateway
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	// handle returned status code
	httpBodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	switch res.StatusCode {
	case 200:
		var task ServiceTask
		if err := json.Unmarshal(httpBodyBytes, &task); err != nil {
			return nil, err
		}
		return &task, nil
	case 400, 500:
		var serr ServiceError
		if err := json.Unmarshal(httpBodyBytes, &serr); err != nil {
			return nil, err
		}
		return nil, &serr
	default:
		return nil, fmt.Errorf("%s", httpBodyBytes)
	}
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
