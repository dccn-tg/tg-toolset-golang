package filer

import (
	"encoding/json"
	"os"
	"testing"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

var (
	netapp Filer
)

const (
	projectID string = "3010000.03"
	groupname string = "tg"
	username  string = "test"
)

func init() {
	filerCfg := NetAppConfig{
		ApiURL:      "https://131.174.44.94",
		ApiUser:     os.Getenv("NETAPP_API_USERNAME"),
		ApiPass:     os.Getenv("NETAPP_API_PASSWORD"),
		Vserver:     "atreides",
		ProjectGID:  1010,
		ProjectUID:  1010,
		ProjectRoot: "/project",
		ProjectMode: "volume",
	}

	netapp = New("netapp", filerCfg)

	logCfg := log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Debug,
	}

	// initialize logger
	log.NewLogger(logCfg, log.InstanceLogrusLogger)
}

func TestUnmarshal(t *testing.T) {
	data := []byte(`{
			"uuid": "27c77b57-a06c-4af5-8c15-1c625e628f64",
			"name": "tg",
			"_links": {
			  "self": {
				"href": "/api/storage/volumes/27c77b57-a06c-4af5-8c15-1c625e628f64"
			  }
			}
		  }`)

	record := Record{}

	json.Unmarshal(data, &record)

	t.Logf("%+v", record)

	records := Records{}
	data = []byte(`{
		"records": [
		  {
			"uuid": "27c77b57-a06c-4af5-8c15-1c625e628f64",
			"name": "tg",
			"_links": {
			  "self": {
				"href": "/api/storage/volumes/27c77b57-a06c-4af5-8c15-1c625e628f64"
			  }
			}
		  }
		],
		"num_records": 1,
		"_links": {
		  "self": {
			"href": "/api/storage/volumes?name=tg"
		  }
		}
	  }`)

	json.Unmarshal(data, &records)
	t.Logf("%+v", records)
}

func TestCreateProject(t *testing.T) {
	if err := netapp.CreateProject(projectID, 10); err != nil {
		t.Errorf("fail to create project volume: %s", err)
	}
}

func TestSetProjectQuota(t *testing.T) {
	if err := netapp.SetProjectQuota(projectID, 20); err != nil {
		t.Errorf("fail to update quota for project %s: %s", projectID, err)
	}
}

func TestCreateHome(t *testing.T) {
	if err := netapp.CreateHome(username, groupname, 10); err != nil {
		t.Errorf("%s\n", err)
	}
}

func TestSetHomeQuota(t *testing.T) {
	if err := netapp.SetHomeQuota(username, groupname, 20); err != nil {
		t.Errorf("%s\n", err)
	}
}

func TestGetHomeQuota(t *testing.T) {
	if quota, err := netapp.GetHomeQuotaInBytes(username, groupname); err != nil {
		t.Errorf("%s\n", err)
	} else {
		t.Logf("quota: %d\n", quota)
	}
}

func TestGetProjectQuota(t *testing.T) {
	if quota, err := netapp.GetProjectQuotaInBytes(projectID); err != nil {
		t.Errorf("%s\n", err)
	} else {
		t.Logf("quota: %d\n", quota)
	}
}
