package config

import (
	"os"
	"testing"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

var testConf Configuration

func init() {
	logCfg := log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Debug,
	}

	// initialize logger
	log.NewLogger(logCfg, log.InstanceLogrusLogger)

	var err error
	testConf, err = LoadConfig(os.Getenv("TG_TOOLSET_CONFIG"))

	if err != nil {
		log.Fatalf("cannot log config file: %s\n", err)
	}
}

func TestYamlKeyMapping(t *testing.T) {

	t.Logf("PDB V2 config: %+v\n", testConf.PDB.V2)

	if testConf.PDB.V2.AuthURL != "https://auth-dev.dccn.nl" {
		t.Errorf("fail to load auth_url from configuration: %s\n", os.Getenv("TG_TOOLSET_CONFIG"))
	}
}

func TestPDBV1Config(t *testing.T) {

	t.Logf("PDB V1 config: %+v\n", testConf.PDB.V1)

	if testConf.PDB.V1.HostSQL != "db.intranet.dccn.nl" {
		t.Errorf("fail to load db_host from configuration: %s\n", os.Getenv("TG_TOOLSET_CONFIG"))
	}
}
