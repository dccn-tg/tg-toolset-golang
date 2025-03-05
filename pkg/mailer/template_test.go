package mailer

import (
	"os"
	"strings"
	"testing"

	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
	"github.com/dccn-tg/tg-toolset-golang/project/pkg/pdb"
)

var cc = "rene.debruin@donders.ru.nl"

func setProtocol() MailerProtocol {
	p := os.Getenv("TEST_MAILER_PROTOCOL")
	switch strings.ToLower(p) {
	case "graph":
		return Graph
	default:
		return SMTP
	}
}

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

	m, err := New(conf.Mailer, setProtocol())
	if err != nil {
		t.Errorf("%s", err)
	}

	data := ProjectAlertTemplateData{
		ProjectID:    "3010000.01",
		ProjectTitle: "test project",
		SenderName:   "DCCN TG Helpdesk",
	}

	data.RecipientName = manager.DisplayName()

	subject, body, err := ComposeMessageFromTemplateFile(os.Getenv("TEST_MAILER_TEMPLATE_NEW"), data)
	if err != nil {
		t.Errorf("%s", err)
	}

	t.Logf("subject: %s", subject)
	t.Logf("body: %s", body)

	if err := m.SendMail("no-reply@donders.ru.nl", subject, body, []string{manager.Email}); err != nil {
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

	m, err := New(conf.Mailer, setProtocol())
	if err != nil {
		t.Errorf("%s", err)
	}

	data := ProjectAlertTemplateData{
		ProjectID:      "3010000.01",
		ProjectTitle:   "test project",
		ProjectEndDate: "2023-04-22",
		SenderName:     "Sabita Raktoe",
	}

	data.RecipientName = manager.DisplayName()
	for _, days := range []int{28, 14, 7, 0} {
		data.ExpiringInDays = days

		subject, body, err := ComposeMessageFromTemplateFile(os.Getenv("TEST_MAILER_TEMPLATE_OOT"), data)

		t.Logf("subject: %s", subject)
		t.Logf("body: %s", body)

		if err != nil {
			t.Errorf("%s", err)
		}

		if err := m.SendMail("sabita.raktoe@donders.ru.nl", subject, body, []string{manager.Email}, cc); err != nil {
			t.Errorf("%s", err)
		}
	}
}

func TestNotifyProjectEndOfGracePeriod(t *testing.T) {

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

	m, err := New(conf.Mailer, setProtocol())
	if err != nil {
		t.Errorf("%s", err)
	}

	data := ProjectAlertTemplateData{
		ProjectID:      "3010000.01",
		ProjectTitle:   "test project",
		ProjectEndDate: "2023-04-22",
		SenderName:     "Sabita Raktoe",
	}

	data.RecipientName = manager.DisplayName()
	data.ExpiringInMonths = -2

	subject, body, err := ComposeMessageFromTemplateFile(os.Getenv("TEST_MAILER_TEMPLATE_EOG"), data)

	t.Logf("subject: %s", subject)
	t.Logf("body: %s", body)

	if err != nil {
		t.Errorf("%s", err)
	}

	if err := m.SendMail("sabita.raktoe@donders.ru.nl", subject, body, []string{manager.Email}, cc); err != nil {
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

	m, err := New(conf.Mailer, setProtocol())
	if err != nil {
		t.Errorf("%s", err)
	}

	data := ProjectAlertTemplateData{
		ProjectID:       "3010000.01",
		ProjectTitle:    "test project",
		QuotaUsageRatio: 95,
		SenderName:      "the TG Helpdesk",
	}

	data.RecipientName = manager.DisplayName()

	subject, body, err := ComposeMessageFromTemplateFile(os.Getenv("TEST_MAILER_TEMPLATE_OOQ"), data)

	if err != nil {
		t.Errorf("%s", err)
	}

	t.Logf("subject: %s", subject)
	t.Logf("body: %s", body)

	if err := m.SendMail("no-reply@donders.ru.nl", subject, body, []string{manager.Email}); err != nil {
		t.Errorf("%s", err)
	}
}
