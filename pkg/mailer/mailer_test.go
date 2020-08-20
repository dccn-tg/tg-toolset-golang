package mailer

import (
	"os"
	"testing"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
)

func TestNotifyProjectProvisioned(t *testing.T) {

	var manager *pdb.User

	manager = &pdb.User{
		Firstname: "Hurng-Chun",
		Lastname:  "Lee",
		Email:     "h.lee@donders.ru.nl",
	}
	pid := "3010000.01"

	conf, err := config.LoadConfig(os.Getenv("TG_TOOLSET_CONFIG"))
	if err != nil {
		t.Errorf("%s", err)
	}

	mailer := New(conf.SMTP)
	if mailer.NotifyProjectProvisioned(*manager, pid); err != nil {
		t.Errorf("%s", err)
	}
}

func TestNotifyUTF8(t *testing.T) {

	var manager *pdb.User

	manager = &pdb.User{
		Firstname: "René",
		Lastname:  "de Bruin",
		Email:     "r.debruin@donders.ru.nl",
	}
	pid := "3010000.01"

	conf, err := config.LoadConfig(os.Getenv("TG_TOOLSET_CONFIG"))
	if err != nil {
		t.Errorf("%s", err)
	}

	mailer := New(conf.SMTP)
	if mailer.NotifyProjectProvisioned(*manager, pid); err != nil {
		t.Errorf("%s", err)
	}
}
