package filergateway

import (
	"os"
	"testing"

	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
	log "github.com/dccn-tg/tg-toolset-golang/pkg/logger"
	"github.com/dccn-tg/tg-toolset-golang/project/pkg/pdb"
)

var testConf config.Configuration
var testCLI NetAppCLI

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
		log.Fatalf("cannot log config file: %s\n", err)
	}

	testCLI = NetAppCLI{Config: testConf.NetAppCLI}
}

func TestCreateProjectQtree(t *testing.T) {

	projectID := "3010000.10"
	data := pdb.DataProjectUpdate{
		Storage: pdb.Storage{
			System:  "netapp",
			QuotaGb: 10,
		},
	}
	err := testCLI.CreateProjectQtree(projectID, &data)
	if err != nil {
		t.Errorf("%s\n", err)
	}
}
