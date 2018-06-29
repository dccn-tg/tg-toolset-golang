package orthanc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func init() {}

var nilTime = (time.Time{}).UnixNano()

// OrthancDateTime defines the datetime structure of Orthanc.
type OrthancDateTime struct {
	time.Time
}

func (ot *OrthancDateTime) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" {
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
	if s == "null" {
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
	if s == "null" {
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

// Patient is the data structure of the DICOM patient information.
type Patient struct {
	ID            string
	IsStable      bool
	LastUPdate    OrthancDateTime
	MainDicomTags DicomTagsPatient
	Studies       []string
	Type          string
}

type DicomTagsPatient struct {
	PatientBirthDate OrthancDate
	PatientID        string
	PatientName      string
	PatientSex       string
}

// Study is the data structure of the DICOM study information.
type Study struct {
	ID                   string
	IsStable             bool
	LastUPdate           OrthancDateTime
	MainDicomTags        DicomTagsStudy
	PatientMainDicomTags DicomTagsPatient
	Series               []string
	Type                 string
}

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

// Series is the data structure of the DICOM series information.
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

type DicomTagsSeries struct {
	BodyPartExamined                  string
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

// GetPatients retrieves the DICOM patients involved in the experiments conducted
// in between a time range.  It returns a channel in which the Patient data objects
// are pushed through.
func GetPatients(from, to time.Time) chan Patient {
	patients := make(chan Patient)

	return patients
}

// GetStudies retrieves the DICOM studies involved in the experiments conducted
// in between a time range.  It returns a channel in which the Study data objects
// are pushed through.
func GetStudies(from, to time.Time) chan Study {
	studies := make(chan Study)

	return studies
}

// GetSerieses retrieves the DICOM serieses involved in the experiments conducted
// in between a time range.  It returns a channel in which the Series data objects
// are pushed through.
func GetSerieses(from, to time.Time) chan Series {
	serieses := make(chan Series)

	return serieses
}
