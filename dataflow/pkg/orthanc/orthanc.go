package orthanc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

var logger *log.Entry
var nilTime = (time.Time{}).UnixNano()

func init() {
	logger = log.WithFields(log.Fields{"source": "orthanc"})
}

// DateTime defines the datetime structure of Orthanc.
type DateTime struct {
	time.Time
}

// UnmarshalJSON converts the complete data-time JSON string of the Orthanc server to the time.Time data object.
func (ot *DateTime) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" || len(s) == 0 {
		ot.Time = time.Time{}
		return
	}
	ot.Time, err = time.Parse("20060102T150405", s)
	return
}

// MarshalJSON converts time.Time data object into the complete JSON date-time string of the Orthanc server.
func (ot *DateTime) MarshalJSON() ([]byte, error) {
	if ot.Time.UnixNano() == nilTime {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", ot.Time.Format("20060102T150405"))), nil
}

func (ot *DateTime) String() string {
	return fmt.Sprintf("%s", ot.Time.Format("20060102T150405"))
}

// IsSet checks whether the DateTime object is set with a time.
func (ot *DateTime) IsSet() bool {
	return ot.UnixNano() != nilTime
}

// Date defines the date structure of Orthanc.
type Date struct {
	time.Time
}

// UnmarshalJSON converts the date part of the data-time JSON string of the Orthanc server to the time.Time data object.
func (ot *Date) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" || len(s) == 0 {
		ot.Time = time.Time{}
		return
	}
	ot.Time, err = time.Parse("20060102", s)
	return
}

// MarshalJSON converts time.Time data object into the date part of the JSON date-time string of the Orthanc server.
func (ot *Date) MarshalJSON() ([]byte, error) {
	if ot.Time.UnixNano() == nilTime {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", ot.Time.Format("20060102"))), nil
}

func (ot *Date) String() string {
	return fmt.Sprintf("%s", ot.Time.Format("20060102"))
}

// IsSet checks whether the Date object is set with a given time.
func (ot *Date) IsSet() bool {
	return ot.UnixNano() != nilTime
}

// Time defines the time structure of Orthanc.
type Time struct {
	time.Time
}

// UnmarshalJSON converts the time part of the data-time JSON string of the Orthanc server to the time.Time data object.
func (ot *Time) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" || len(s) == 0 {
		ot.Time = time.Time{}
		return
	}
	ot.Time, err = time.Parse("150405", s[:6])
	return
}

// MarshalJSON converts time.Time data object into the time part of the JSON date-time string of the Orthanc server.
func (ot *Time) MarshalJSON() ([]byte, error) {
	if ot.Time.UnixNano() == nilTime {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", ot.Time.Format("150405"))), nil
}

func (ot *Time) String() string {
	return fmt.Sprintf("%s", ot.Time.Format("150405"))
}

// IsSet checks whether the Time object is set with a given time.
func (ot *Time) IsSet() bool {
	return ot.UnixNano() != nilTime
}

// Patient is the data structure of the Orthanc attributes for a DICOM patient.
//
// Note that the LastUpdate is in UTC.
type Patient struct {
	ID            string
	IsStable      bool
	LastUpdate    DateTime
	MainDicomTags DicomTagsPatient
	Studies       []string
	Type          string
}

// DicomTagsPatient is the data structure of a few DICOM-header attributes extracted by Orthanc for a DICOM patient.
type DicomTagsPatient struct {
	PatientBirthDate Date
	PatientID        string
	PatientName      string
	PatientSex       string
}

// Study is the data structure of the Orthanc attributes for a DICOM study.
//
// Note that the LastUpdate is in UTC.
type Study struct {
	ID                   string
	IsStable             bool
	LastUpdate           DateTime
	MainDicomTags        DicomTagsStudy
	PatientMainDicomTags DicomTagsPatient
	Series               []string
	Type                 string
}

// DicomTagsStudy is the data structure of a few DICOM-header attributes extracted by Orthanc for a DICOM study.
type DicomTagsStudy struct {
	AccessionNumber               string
	InstitutionNuame              string
	ReferringPhysicianName        string
	RequestedProcedureDescription string
	RequestingPhysician           string
	StudyDate                     Date
	StudyDescription              string
	StudyID                       string
	StudyInstanceUID              string
	StudyTime                     Time
}

// Series is the data structure of the Orthanc attributes for a DICOM series.
//
// Note that the LastUpdate is in UTC.
type Series struct {
	ID                        string
	ExpectedNumberOfInstances string
	IsStable                  bool
	LastUpdate                DateTime
	MainDicomTags             DicomTagsSeries
	ParentStudy               string
	Status                    string
	Type                      string
	Instances                 []string
}

// DicomTagsSeries is the data structure of a few DICOM-header attributes extracted by Orthanc for a DICOM series.
type DicomTagsSeries struct {
	BodyPartExamined                  string
	CardiacNumberOfImages             string
	ImageOrientationPatient           string
	Manufacturer                      string
	Modality                          string
	PerformedProcedureStepDescription string
	ProtocolName                      string
	SequenceName                      string
	SeriesDate                        Date
	SeriesDescription                 string
	SeriesInstanceUID                 string
	SeriesNumber                      string
	SeriesTime                        Time
	StationName                       string
}

// Orthanc defines the object for connecting to the Orthanc service.
type Orthanc struct {
	PrefixURL string
	Username  string
	Password  string
}

// DicomObject is a enumeratable integer referring to one of DICOM objects.
type DicomObject int

// Valid DicomObjects are listed below:
//
// DicomPatient: a subject
//
// DicomStudy: a study on a subject
//
// DicomSeries: a series within a study
//
// DicomInstance: a image instance within a series
const (
	DicomPatient DicomObject = iota
	DicomStudy
	DicomSeries
	DicomInstance
)

func (o DicomObject) String() string {
	names := []string{
		"Patient",
		"Study",
		"Series",
		"Instance",
	}
	return names[o]
}

// DicomQuery defines the c-Find attributes for the finding
// DICOM objects.
type DicomQuery struct {
	StudyDate string
}

// Query defines the query data accepted by the Orthanc's
// /tools/find interface.
type Query struct {
	Level string
	Query DicomQuery
}

// getJSON decodes the JSON output from getting the suffixURL, and converts
// the output to the destination target structure.
func (o Orthanc) getJSON(suffixURL string, target interface{}) error {

	// set connection timeout
	c := &http.Client{Timeout: 10 * time.Second}

	// prepare request with username/password
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", o.PrefixURL, suffixURL), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(o.Username, o.Password)

	// send request and retrive response body
	rsp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	// decode response body into target object
	return json.NewDecoder(rsp.Body).Decode(target)
}

func (o Orthanc) postJSON(suffixURL string, data string, target interface{}) error {
	// set connection timeout
	c := &http.Client{Timeout: 10 * time.Second}

	// prepare request with username/password
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s", o.PrefixURL, suffixURL), strings.NewReader(data))
	if err != nil {
		return err
	}
	req.SetBasicAuth(o.Username, o.Password)

	// send request and retrive response body
	rsp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	// decode response body into target object
	return json.NewDecoder(rsp.Body).Decode(target)
}

// GetPatient retrieves the DICOM patient information from the Orthanc server,
// and returns the Patient data object.
func (o Orthanc) GetPatient(id string) (patient Patient, err error) {
	patient = Patient{}
	err = o.getJSON(fmt.Sprintf("patients/%s", id), &patient)
	return
}

// GetStudy retrieves the DICOM study information from the Orthanc server,
// and returns the Study data object.
func (o Orthanc) GetStudy(id string) (study Study, err error) {
	study = Study{}
	err = o.getJSON(fmt.Sprintf("studies/%s", id), &study)
	return
}

// GetSeries retrieves the DICOM series information from the Orthanc server,
// and returns the Series data object.
func (o Orthanc) GetSeries(id string) (series Series, err error) {
	series = Series{}
	err = o.getJSON(fmt.Sprintf("series/%s", id), &series)
	return
}

// GetStudies retrieves the DICOM studies involved in the experiments conducted
// in between a time range.  It returns a channel in which the Study data objects
// are pushed through.
func (o Orthanc) GetStudies(from, to time.Time) (studies []Study, err error) {
	ids := []string{}
	err = o.getJSON("studies", &ids)
	if err != nil {
		return
	}
	// filling up the internal work channel for retrieving study details
	nworkers := 4
	wchan := make(chan string, 2*nworkers)
	go func() {
		for _, id := range ids {
			wchan <- id
		}
		close(wchan)
	}()

	var wg sync.WaitGroup
	wg.Add(nworkers)

	// go routines to retrieve series details in parallel
	for i := 0; i < nworkers; i++ {
		go func() {
			for {
				_id, opened := <-wchan
				if !opened {
					break
				}

				s, err := o.GetStudy(_id)
				if err != nil {
					logger.Errorf("cannot get study: %s, error: %+v\n", _id, err)
					continue
				}
				// check if the series's datetime is between the requested time range.
				ds := s.MainDicomTags.StudyDate
				ts := s.MainDicomTags.StudyTime
				dts := time.Date(ds.Year(), ds.Month(), ds.Day(), ts.Hour(), ts.Minute(), ts.Second(), 0, from.Location())
				if !dts.After(from) || !dts.Before(to) {
					//logger.Errorf("study skipped due to time range: %s, time: %+v\n", _id, dts)
					continue
				}
				logger.Debugf("id: %s, time: %+v\n", s.ID, dts)
				studies = append(studies, s)
			}
			wg.Done()
		}()
	}

	// wait for all workers to finish
	wg.Wait()

	return
}

// GetSerieses retrieves the DICOM serieses involved in the experiments conducted
// in between a time range.  It returns a channel in which the Series data objects
// are pushed through.
func (o Orthanc) GetSerieses(from, to time.Time) (serieses []Series, err error) {
	ids := []string{}
	err = o.getJSON("series", &ids)
	if err != nil {
		return
	}

	// filling up the internal work channel for retrieving series details
	nworkers := 4
	wchan := make(chan string, 2*nworkers)
	go func() {
		for _, id := range ids {
			wchan <- id
		}
		close(wchan)
	}()

	var wg sync.WaitGroup
	wg.Add(nworkers)

	// go routines to retrieve series details in parallel
	for i := 0; i < nworkers; i++ {
		go func() {
			for {
				_id, opened := <-wchan
				if !opened {
					break
				}

				s, err := o.GetSeries(_id)
				if err != nil {
					logger.Errorf("cannot get series: %s, error: %+v\n", _id, err)
					continue
				}
				// check if the series's datetime is between the requested time range.
				ds := s.MainDicomTags.SeriesDate
				ts := s.MainDicomTags.SeriesTime
				dts := time.Date(ds.Year(), ds.Month(), ds.Day(), ts.Hour(), ts.Minute(), ts.Second(), 0, from.Location())
				if !dts.After(from) || !dts.Before(to) {
					//logger.Errorf("series skipped due to time range: %s, time: %+v\n", _id, dts)
					continue
				}
				logger.Debugf("id: %s, time: %+v\n", s.ID, dts)
				serieses = append(serieses, s)
			}
			wg.Done()
		}()
	}

	// wait for all workers to finish
	wg.Wait()

	return
}

// ListObjectIDs uses Orthanc's /tools/find interface to retrieve a list of
// DICOM object IDs between the given time range.
func (o Orthanc) ListObjectIDs(level DicomObject, from, to time.Time) (ids []string, err error) {

	qry := Query{
		Level: fmt.Sprintf("%s", level),
		Query: DicomQuery{
			StudyDate: fmt.Sprintf("%d%02d%02d-%d%02d%02d", from.Year(), from.Month(), from.Day(), to.Year(), to.Month(), to.Day()),
		},
	}

	qryJSON, err := json.Marshal(qry)

	fmt.Println(string(qryJSON))

	if err != nil {
		return
	}

	err = o.postJSON("tools/find", string(qryJSON), &ids)
	if err != nil {
		return
	}

	return
}
