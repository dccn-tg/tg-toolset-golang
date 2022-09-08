package pdb

import (
	"os"
	"testing"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

var testConf config.Configuration
var testPDB PDB

func init() {
	logCfg := log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Debug,
	}

	// initialize logger
	log.NewLogger(logCfg, log.InstanceLogrusLogger)

	var err error
	testConf, err = config.LoadConfig(os.Getenv("TG_TOOLSET_CONFIG"))

	if err != nil {
		log.Fatalf("cannot load config file: %s\n", err)
	}

	testPDB, err = New(testConf.PDB)

	if err != nil {
		log.Fatalf("cannot load operator: %s\n", err)
	}
}

func TestGetProject(t *testing.T) {
	p, err := testPDB.GetProject("3010000.01")
	if err != nil {
		t.Errorf("%s\n", err)
	}
	t.Logf("%+v", p)
}

func TestGetProjectPendingActions(t *testing.T) {
	acts, err := testPDB.GetProjectPendingActions()
	if err != nil {
		t.Errorf("%s\n", err)
	}
	t.Logf("pending actions: %+v\n", acts)
}

func TestGetExperimentersForSharedAnatomicalMR(t *testing.T) {
	users, err := testPDB.GetExperimentersForSharedAnatomicalMR()
	if err != nil {
		t.Errorf("%s\n", err)
	}
	t.Logf("%d experimenters: \n", len(users))
	for _, u := range users {
		t.Logf("%+v\n", u)
	}
}

// func TestDelProjectPendingActions(t *testing.T) {
// 	// get pending actions
// 	acts, err := testPDB.GetProjectPendingActions()
// 	if err != nil {
// 		t.Errorf("%s\n", err)
// 	}
// 	t.Logf("pending actions: %+v\n", acts)

// 	// delete pending actions
// 	err = testPDB.DelProjectPendingActions(acts)
// 	if err != nil {
// 		t.Errorf("%s\n", err)
// 	}
// }
