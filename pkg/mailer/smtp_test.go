package mailer

import (
	"os"
	"testing"

	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
)

func TestSMTPMailer(t *testing.T) {

	emails := []string{
		"h.lee@donders.ru.nl",
	}

	conf, err := config.LoadConfig(os.Getenv("TG_TOOLSET_CONFIG"))
	if err != nil {
		t.Errorf("%s", err)
	}

	m, err := New(conf.Mailer, SMTP)
	if err != nil {
		t.Errorf("%s", err)
	}

	subject := "test SMTP mail sending"
	body := "A test message here"

	if err := m.SendMail("no-reply@donders.ru.nl", subject, body, emails); err != nil {
		t.Errorf("%s", err)
	}
}
