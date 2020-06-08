// Package mailer implements email notifications using the SMTP server.
package mailer

import (
	"bytes"
	"fmt"
	"net/smtp"
	"text/template"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
)

// New returns a new mailer instance.
func New(config config.SMTPConfiguration) *Mailer {
	return &Mailer{config: config}
}

// Mailer implements varias email notifications.
type Mailer struct {
	config config.SMTPConfiguration
}

// NotifyProjectProvisioned sends out email notification
// to `manager` about the just provisioned project `pid`.
func (m *Mailer) NotifyProjectProvisioned(manager pdb.User, pid string) error {

	from := "helpdesk@fcdonders.ru.nl"
	name := fmt.Sprintf("%s %s", manager.Firstname, manager.Lastname)
	subject := fmt.Sprintf("Storage of your project %s has been initalized", pid)

	// message template
	tempStr := `Dear {{.Name}},

The storage of your project {{.ProjectID}} has been initialised.
	
You may now access the storage via the following paths:
	
	* on Windows desktop: P:\{{.ProjectID}}
	* in the cluster: /project/{{.ProjectID}}
	
For managing data access permission for project collaborators, please follow the guide:
	
	http://dccn-hpc-wiki.readthedocs.io/en/latest/docs/project_storage/access_management.html
	
For more information about the project storage, please refer to
	
	https://intranet.donders.ru.nl/index.php?id=4733
	
Should you have any questions, please don't hesitate to contact the TG helpdesk <helpdesk@fcdonders.ru.nl>.
	
Best regards, the DCCN Technical Group`

	// data for message template
	tempData := struct {
		Name      string
		ProjectID string
	}{name, pid}

	body, err := composeMessageTempstr(tempStr, tempData)

	if err != nil {
		return err
	}

	return sendMail(m.config, from, manager.Email, subject, body)
}

// composeMessage composes a message using the given `tempfile` template file and the `data`
// provided.
func composeMessageTempfile(tempfile string, data interface{}) (string, error) {
	var buf bytes.Buffer
	t := template.Must(template.New("message").ParseFiles([]string{tempfile}...))
	err := t.Execute(&buf, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// composeMessage composes a message using the given `tempstr` template string and the `data`
// provided.
func composeMessageTempstr(tempstr string, data interface{}) (string, error) {
	var buf bytes.Buffer
	t := template.Must(template.New("message").Parse(tempstr))
	err := t.Execute(&buf, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// sendMail sends out a email with given `from`, `to`, `subject` and `body` content
// using the SMTP server configuration provided by `config`.
func sendMail(config config.SMTPConfiguration, from, to, subject, body string) error {

	// SMTP server address
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	// RFC-822 style email message
	msg := []byte("Subject: " + subject + "\r\n" +
		body + "\r\n")

	// SMTP plain auth with username/password
	if config.AuthPlainUser != "" && config.AuthPlainPass != "" {
		auth := smtp.PlainAuth("", config.AuthPlainUser, config.AuthPlainPass, config.Host)
		return smtp.SendMail(addr, auth, from, []string{to}, msg)
	}

	// no SMTP authentication
	return smtp.SendMail(addr, nil, from, []string{to}, msg)
}
