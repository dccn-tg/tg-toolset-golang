package pdb2

import (
	"os"
	"testing"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

func init() {
	logCfg := log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Debug,
	}

	// initialize logger
	log.NewLogger(logCfg, log.InstanceLogrusLogger)
}

func TestGetAuthToken(t *testing.T) {

	PDB_CORE_API_URL = os.Getenv("PDB_CORE_API_URL")
	AUTH_SERVER_URL = os.Getenv("AUTH_SERVER_URL")

	token, err := getAuthToken(os.Getenv("PDB_CORE_API_CLIENT_SECRET"))
	if err != nil {
		t.Errorf("%s\n", err)
	}
	t.Logf("token: %+v\n", token)
}

func TestGetProjectPendingRoles(t *testing.T) {
	PDB_CORE_API_URL = os.Getenv("PDB_CORE_API_URL")
	AUTH_SERVER_URL = os.Getenv("AUTH_SERVER_URL")

	pendingRoles, err := getProjectPendingRoles(os.Getenv("PDB_CORE_API_CLIENT_SECRET"))
	if err != nil {
		t.Errorf("%s\n", err)
	}
	t.Logf("pendingRoles: %+v\n", pendingRoles)
}

func TestGetProjectStorageResource(t *testing.T) {
	PDB_CORE_API_URL = os.Getenv("PDB_CORE_API_URL")
	AUTH_SERVER_URL = os.Getenv("AUTH_SERVER_URL")

	stor, err := getProjectStorageResource(os.Getenv("PDB_CORE_API_CLIENT_SECRET"), "3010000.03")
	if err != nil {
		t.Errorf("%s\n", err)
	}
	t.Logf("storage resource: %+v\n", stor)
}
