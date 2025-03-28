package mailer

import (
	"os"
	"testing"

	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
)

func TestSMTPMailer(t *testing.T) {

	emails := []string{
		os.Getenv("TEST_MAILER_TO_ADDRESS"),
	}

	conf, err := config.LoadConfig(os.Getenv("TG_TOOLSET_CONFIG"))
	if err != nil {
		t.Errorf("%s", err)
	}

	m, err := New(conf.Mailer, SMTP)
	if err != nil {
		t.Errorf("%s", err)
	}

	subject := "test SMTP plain-text mail"
	body := "A test message here"

	if err := m.SendMail("no-reply@donders.ru.nl", subject, body, emails); err != nil {
		t.Errorf("%s", err)
	}

	subject = "test SMTP html-text mail"
	body = `<head>TEST</head>
<body>
<h1>HTML message</h1>
<p style="color:red;">This is a html paragraph</p>
</body>`

	if err := m.SendHtmlMail("no-reply@donders.ru.nl", subject, body, emails); err != nil {
		t.Errorf("%s", err)
	}
}
