package orthanc

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"dccn.nl/config"
	"github.com/spf13/viper"
)

var conf config.Configuration

func init() {
	// load configuration
	cfg := filepath.Join(os.Getenv("GOPATH"), "etc/config_test.yml")
	viper.SetConfigFile(cfg)
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Sprintf("Error reading config file, %s", err))
	}
	err := viper.Unmarshal(&conf)
	if err != nil {
		panic(fmt.Sprintf("unable to decode into struct, %s", err))
	}
}

func TestGetPatient(t *testing.T) {

	o := Orthanc{
		PrefixURL: conf.PACS.PrefixURL,
		Username:  conf.PACS.Username,
		Password:  conf.PACS.Password,
	}

	p, err := o.GetPatient("ea76e883-67f05627-1921e926-80031298-3a2b9712")
	if err != nil {
		t.Errorf("Fail getting patient: %+v", err)
	}

	t.Logf("%+v", p)
}
func TestGetStudy(t *testing.T) {

	o := Orthanc{
		PrefixURL: conf.PACS.PrefixURL,
		Username:  conf.PACS.Username,
		Password:  conf.PACS.Password,
	}

	s, err := o.GetStudy("97da70e3-34346a6d-88fb16d9-883d8198-dfb6184c")
	if err != nil {
		t.Errorf("Fail getting study: %+v", err)
	}

	t.Logf("%+v", s)
}

func TestGetSeries(t *testing.T) {

	o := Orthanc{
		PrefixURL: conf.PACS.PrefixURL,
		Username:  conf.PACS.Username,
		Password:  conf.PACS.Password,
	}

	s, err := o.GetSeries("1a6376eb-5a349edb-e36216e4-b2502f85-f84e1b48")
	if err != nil {
		t.Errorf("Fail getting study: %+v", err)
	}

	t.Logf("%+v", s)
}
