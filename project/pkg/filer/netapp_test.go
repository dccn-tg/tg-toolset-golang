package filer

import (
	"encoding/json"
	"testing"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

var netapp NetApp

func init() {
	netapp = NetApp{
		APIServerURL: "https://131.174.44.94",
		APIUsername:  "",
		APIPassword:  "",
		Vserver:      "atreides",
		ProjectGID:   1010,
		ProjectUID:   1010,
		ProjectRoot:  "/project",
	}

	cfg := log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Debug,
	}

	// initialize logger
	log.NewLogger(cfg, log.InstanceLogrusLogger)
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

// func TestWaitJob(t *testing.T) {

// 	l := Link{}
// 	l.Self.Href = "/api/cluster/jobs/54683337-729e-11ea-98ba-00a0989c4283"

// 	j := APIJob{
// 		Job: Job{
// 			Link: &l,
// 		},
// 	}

// 	netapp.waitJob(&j)
// }

func TestGetObjectByName(t *testing.T) {

	vol := Volume{}

	if err := netapp.GetObjectByName(netapp.volName("3010000.01"), "/storage/volumes", &vol); err != nil {
		t.Errorf("fail to get object: %s", err)
	}

	t.Logf("retrieved volume: %+v", vol)
}

func TestCreateProject(t *testing.T) {
	netapp.ProjectMode = "volume"
	if err := netapp.CreateProject("3010000.03", 10); err != nil {
		t.Errorf("fail to create project volume: %s", err)
	}
}
