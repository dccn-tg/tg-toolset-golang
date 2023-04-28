package pdbutil

import (
	"os"
	"testing"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	"github.com/Donders-Institute/tg-toolset-golang/pkg/mailer"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
)

func TestNotifyProjectProvisioned(t *testing.T) {

	var manager = &pdb.User{
		Firstname:  "Hürng-Chun",
		Lastname:   "Lee",
		Middlename: "",
		Email:      "h.lee@donders.ru.nl",
	}

	conf, err := config.LoadConfig(os.Getenv("TG_TOOLSET_CONFIG"))
	if err != nil {
		t.Errorf("%s", err)
	}

	mailer := mailer.New(conf.SMTP)

	data := ProjectAlertTemplateData{
		ProjectID:    "3010000.01",
		ProjectTitle: "test project",
		SenderName:   "DCCN TG Helpdesk",
	}

	data.RecipientName = manager.DisplayName()

	subject, body, err := ComposeProjectProvisionedAlert(data)
	if err != nil {
		t.Errorf("%s", err)
	}

	t.Logf("subject: %s", subject)
	t.Logf("body: %s", body)

	if mailer.SendMail("no-reply@donders.ru.nl", manager.Email, subject, body); err != nil {
		t.Errorf("%s", err)
	}
}

func TestNotifyProjectExpiring(t *testing.T) {

	var manager = &pdb.User{
		Firstname:  "Hürng-Chun",
		Lastname:   "Lee",
		Middlename: "",
		Email:      "h.lee@donders.ru.nl",
	}

	conf, err := config.LoadConfig(os.Getenv("TG_TOOLSET_CONFIG"))
	if err != nil {
		t.Errorf("%s", err)
	}

	mailer := mailer.New(conf.SMTP)

	data := ProjectAlertTemplateData{
		ProjectID:      "3010000.01",
		ProjectTitle:   "test project",
		ProjectEndDate: "2023-04-22",
		SenderName:     "Sabita Raktoe",
	}

	data.RecipientName = manager.DisplayName()
	for _, days := range []int{28, 14, 0} {
		data.ExpiringInDays = days
		subject, body, err := ComposeProjectExpiringAlert(data)

		t.Logf("subject: %s", subject)
		t.Logf("body: %s", body)

		if err != nil {
			t.Errorf("%s", err)
		}

		if mailer.SendMail("sabita.raktoe@donders.ru.nl", manager.Email, subject, body); err != nil {
			t.Errorf("%s", err)
		}
	}
}

func TestNotifyProjectExpired(t *testing.T) {

	var manager = &pdb.User{
		Firstname:  "Hürng-Chun",
		Lastname:   "Lee",
		Middlename: "",
		Email:      "h.lee@donders.ru.nl",
	}

	conf, err := config.LoadConfig(os.Getenv("TG_TOOLSET_CONFIG"))
	if err != nil {
		t.Errorf("%s", err)
	}

	mailer := mailer.New(conf.SMTP)

	data := ProjectAlertTemplateData{
		ProjectID:      "3010000.01",
		ProjectTitle:   "test project",
		ProjectEndDate: "2023-04-22",
		SenderName:     "Sabita Raktoe",
	}

	data.RecipientName = manager.DisplayName()

	subject, body, err := ComposeProjectExpiredAlert(data)
	if err != nil {
		t.Errorf("%s", err)
	}

	t.Logf("subject: %s", subject)
	t.Logf("body: %s", body)

	if mailer.SendMail("sabita.raktoe@donders.ru.nl", manager.Email, subject, body); err != nil {
		t.Errorf("%s", err)
	}
}

func TestNotifyProjectOutOfQuota(t *testing.T) {

	var manager = &pdb.User{
		Firstname:  "Hürng-Chun",
		Lastname:   "Lee",
		Middlename: "",
		Email:      "h.lee@donders.ru.nl",
	}

	conf, err := config.LoadConfig(os.Getenv("TG_TOOLSET_CONFIG"))
	if err != nil {
		t.Errorf("%s", err)
	}

	mailer := mailer.New(conf.SMTP)

	data := ProjectAlertTemplateData{
		ProjectID:       "3010000.01",
		ProjectTitle:    "test project",
		QuotaUsageRatio: 95,
		SenderName:      "the TG Helpdesk",
	}

	data.RecipientName = manager.DisplayName()

	subject, body, err := ComposeProjectOutOfQuotaAlert(data)
	if err != nil {
		t.Errorf("%s", err)
	}

	t.Logf("subject: %s", subject)
	t.Logf("body: %s", body)

	if mailer.SendMail("no-reply@donders.ru.nl", manager.Email, subject, body); err != nil {
		t.Errorf("%s", err)
	}
}
