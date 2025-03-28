package mailer

import (
	"os"
	"testing"

	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
)

func TestGraphMailer(t *testing.T) {

	emails := []string{
		"h.lee@donders.ru.nl",
	}

	conf, err := config.LoadConfig(os.Getenv("TG_TOOLSET_CONFIG"))
	if err != nil {
		t.Errorf("%s", err)
	}

	m, err := New(conf.Mailer, Graph)
	if err != nil {
		t.Errorf("%s", err)
	}

	subject := "test msgraph plain-text mail"
	body := "A test message here"

	if err := m.SendMail(os.Getenv("TEST_MAILER_GRAPH_UPN"), subject, body, emails); err != nil {
		t.Errorf("%s", err)
	}

	subject = "test msgraph html-text mail"
	body = `<head>TEST</head>
<body>
<h1>HTML message</h1>
<p style="color:red;">This is a html paragraph</p>
</body>`

	if err := m.SendHtmlMail(os.Getenv("TEST_MAILER_GRAPH_UPN"), subject, body, emails); err != nil {
		t.Errorf("%s", err)
	}

}
