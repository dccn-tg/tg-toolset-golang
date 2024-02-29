package pdb

// Run the test with the following command and env. variables
//
// $ TEST_PROJECT_NUMBER=3010000.01 \
//   TEST_USERNAME=username \
//   TEST_EMAIL=e.mail@donders.ru.nl \
//   TEST_BOOKING_DATE=2023-04-28 \
//   TEST_CONFIG=/path/of/config/file \
//   go test -v github.com/dccn-tg/tg-toolset-golang/project/pkg/pdb/...

import (
	"math"
	"os"
	"testing"

	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
	log "github.com/dccn-tg/tg-toolset-golang/pkg/logger"
)

var testConf config.Configuration
var testPDB PDB

var (
	configFile    = os.Getenv("TEST_CONFIG")
	projectNumber = os.Getenv("TEST_PROJECT_NUMBER")
	username      = os.Getenv("TEST_USERNAME")
	userEmail     = os.Getenv("TEST_EMAIL")
	bookingDate   = os.Getenv("TEST_BOOKING_DATE")
)

func init() {
	logCfg := log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Debug,
	}

	// initialize logger
	log.NewLogger(logCfg, log.InstanceLogrusLogger)

	var err error
	testConf, err = config.LoadConfig(configFile)

	if err != nil {
		log.Fatalf("cannot load config file: %s\n", err)
	}

	testPDB, err = New(testConf.PDB)

	if err != nil {
		log.Fatalf("cannot load operator: %s\n", err)
	}
}

func TestGetUsers(t *testing.T) {
	users, err := testPDB.GetUsers(true)
	if err != nil {
		t.Errorf("%s\n", err)
	}

	t.Logf("%d users\n", len(users))

	for i := 0; i < int(math.Min(3, float64(len(users)))); i++ {
		t.Logf("%d: %+v\n", i, users[i])
	}
}

func TestGetProjects(t *testing.T) {
	prjs, err := testPDB.GetProjects(true)
	if err != nil {
		t.Errorf("%s\n", err)
	}

	t.Logf("%d projects\n", len(prjs))

	for i := 0; i < int(math.Min(3, float64(len(prjs)))); i++ {
		t.Logf("%d: %+v\n", i, prjs[i])
	}
}

func TestGetProject(t *testing.T) {
	p, err := testPDB.GetProject(projectNumber)
	if err != nil {
		t.Errorf("%s\n", err)
	}
	t.Logf("%+v", p)
}

func TestGetUser(t *testing.T) {
	u, err := testPDB.GetUser(username)
	if err != nil {
		t.Errorf("%s\n", err)
	}
	t.Logf("%+v", u)
}

func TestGetUserByEmail(t *testing.T) {
	u, err := testPDB.GetUserByEmail(userEmail)
	if err != nil {
		t.Errorf("%s\n", err)
	}
	t.Logf("%+v", u)
}

// func TestGetProjectPendingActions(t *testing.T) {
// 	acts, err := testPDB.GetProjectPendingActions()
// 	if err != nil {
// 		t.Errorf("%s\n", err)
// 	}
// 	t.Logf("pending actions: %+v\n", acts)
// }

// func TestGetExperimentersForSharedAnatomicalMR(t *testing.T) {
// 	users, err := testPDB.GetExperimentersForSharedAnatomicalMR()
// 	if err != nil {
// 		t.Errorf("%s\n", err)
// 	}
// 	t.Logf("%d experimenters: \n", len(users))
// 	for _, u := range users {
// 		t.Logf("%+v\n", u)
// 	}
// }

func TestGetLabBookings(t *testing.T) {

	bookings, err := testPDB.GetLabBookingsForWorklist(MRI, bookingDate)
	if err != nil {
		t.Errorf("%s\n", err)
	}
	t.Logf("%d bookings: \n", len(bookings))
	for i := 0; i < int(math.Min(3, float64(len(bookings)))); i++ {
		t.Logf("%d: %+v\n", i, bookings[i])
	}
}
