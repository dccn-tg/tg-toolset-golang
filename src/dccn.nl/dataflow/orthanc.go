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

// OrthancDateTime defines the datetime structure of Orthanc.
type OrthancDateTime struct {
	time.Time
}

func (ot *OrthancDateTime) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" || len(s) == 0 {
		ot.Time = time.Time{}
		return
	}
	ot.Time, err = time.Parse("20060102T150405", s)
	return
}

func (ot *OrthancDateTime) MarshalJSON() ([]byte, error) {
	if ot.Time.UnixNano() == nilTime {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", ot.Time.Format("20060102T150405"))), nil
}

func (ot *OrthancDateTime) String() string {
	return fmt.Sprintf("%s", ot.Time.Format("20060102T150405"))
}

func (ot *OrthancDateTime) IsSet() bool {
	return ot.UnixNano() != nilTime
}

// OrthancDate defines the date structure of Orthanc.
type OrthancDate struct {
	time.Time
}

func (ot *OrthancDate) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" || len(s) == 0 {
		ot.Time = time.Time{}
		return
	}
	ot.Time, err = time.Parse("20060102", s)
	return
}

func (ot *OrthancDate) MarshalJSON() ([]byte, error) {
	if ot.Time.UnixNano() == nilTime {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", ot.Time.Format("20060102"))), nil
}

func (ot *OrthancDate) String() string {
	return fmt.Sprintf("%s", ot.Time.Format("20060102"))
}

func (ot *OrthancDate) IsSet() bool {
	return ot.UnixNano() != nilTime
}

// OrthancTime defines the time structure of Orthanc.
type OrthancTime struct {
	time.Time
}

func (ot *OrthancTime) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" || len(s) == 0 {
		ot.Time = time.Time{}
		return
	}
	ot.Time, err = time.Parse("150405", s[:6])
	return
}

func (ot *OrthancTime) MarshalJSON() ([]byte, error) {
	if ot.Time.UnixNano() == nilTime {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", ot.Time.Format("150405"))), nil
}

func (ot *OrthancTime) String() string {
	return fmt.Sprintf("%s", ot.Time.Format("150405"))
}

func (ot *OrthancTime) IsSet() bool {
	return ot.UnixNano() != nilTime
}

// Patient is the data structure of the Orthanc attributes for a DICOM patient.
//
// Note that the LastUpdate is in UTC.
type Patient struct {
	ID            string
	IsStable      bool
	LastUpdate    OrthancDateTime
	MainDicomTags DicomTagsPatient
	Studies       []string
	Type          string
}

// DicomTagsPatient is the data structure of a few DICOM-header attributes extracted by Orthanc for a DICOM patient.
type DicomTagsPatient struct {
	PatientBirthDate OrthancDate
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
	LastUpdate           OrthancDateTime
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
	StudyDate                     OrthancDate
	StudyDescription              string
	StudyID                       string
	StudyInstanceUID              string
	StudyTime                     OrthancTime
}

// Series is the data structure of the Orthanc attributes for a DICOM series.
//
// Note that the LastUpdate is in UTC.
type Series struct {
	ID                        string
	ExpectedNumberOfInstances string
	IsStable                  bool
	LastUpdate                OrthancDateTime
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
	SeriesDate                        OrthancDate
	SeriesDescription                 string
	SeriesInstanceUID                 string
	SeriesNumber                      string
	SeriesTime                        OrthancTime
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

// OrthancQuery defines the query data accepted by the Orthanc's
// /tools/find interface.
type OrthancQuery struct {
	Level string
	Query DicomQuery
}

// getJson decodes the JSON output from getting the suffixURL, and converts
// the output to the destination target structure.
func (o Orthanc) getJson(suffixURL string, target interface{}) error {

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

func (o Orthanc) postJson(suffixURL string, data string, target interface{}) error {
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
	err = o.getJson(fmt.Sprintf("patients/%s", id), &patient)
	return
}

// GetStudy retrieves the DICOM study information from the Orthanc server,
// and returns the Study data object.
func (o Orthanc) GetStudy(id string) (study Study, err error) {
	study = Study{}
	err = o.getJson(fmt.Sprintf("studies/%s", id), &study)
	return
}

// GetSeries retrieves the DICOM series information from the Orthanc server,
// and returns the Series data object.
func (o Orthanc) GetSeries(id string) (series Series, err error) {
	series = Series{}
	err = o.getJson(fmt.Sprintf("series/%s", id), &series)
	return
}

// GetStudies retrieves the DICOM studies involved in the experiments conducted
// in between a time range.  It returns a channel in which the Study data objects
// are pushed through.
func (o Orthanc) GetStudies(from, to time.Time) (studies []Study, err error) {
	ids := []string{}
	err = o.getJson("studies", &ids)
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
				d_s := s.MainDicomTags.StudyDate
				t_s := s.MainDicomTags.StudyTime
				dt_s := time.Date(d_s.Year(), d_s.Month(), d_s.Day(), t_s.Hour(), t_s.Minute(), t_s.Second(), 0, from.Location())
				if !dt_s.After(from) || !dt_s.Before(to) {
					//logger.Errorf("study skipped due to time range: %s, time: %+v\n", _id, dt_s)
					continue
				}
				logger.Debugf("id: %s, time: %+v\n", s.ID, dt_s)
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
	err = o.getJson("series", &ids)
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
				d_s := s.MainDicomTags.SeriesDate
				t_s := s.MainDicomTags.SeriesTime
				dt_s := time.Date(d_s.Year(), d_s.Month(), d_s.Day(), t_s.Hour(), t_s.Minute(), t_s.Second(), 0, from.Location())
				if !dt_s.After(from) || !dt_s.Before(to) {
					//logger.Errorf("series skipped due to time range: %s, time: %+v\n", _id, dt_s)
					continue
				}
				logger.Debugf("id: %s, time: %+v\n", s.ID, dt_s)
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

	qry := OrthancQuery{
		Level: fmt.Sprintf("%s", level),
		Query: DicomQuery{
			StudyDate: fmt.Sprintf("%d%02d%02d-%d%02d%02d", from.Year(), from.Month(), from.Day(), to.Year(), to.Month(), to.Day()),
		},
	}

	qryJson, err := json.Marshal(qry)

	fmt.Println(string(qryJson))

	if err != nil {
		return
	}

	err = o.postJson("tools/find", string(qryJson), &ids)
	if err != nil {
		return
	}

	return
}
