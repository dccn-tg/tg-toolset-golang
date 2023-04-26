// Package mailer implements email notifications using the SMTP server.
package mailer

import (
	"bytes"
	"encoding/base64"
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

// AlertProjectStorageOot sends out alert email concerning project (about to) running out-of-time (i.e. about to expire)
func (m *Mailer) AlertProjectStorageOot(recipient pdb.User, penddate, pid, pname string) error {
	from := "sabita.raktoe@donders.ru.nl"
	name := fmt.Sprintf("%s %s", recipient.Firstname, recipient.Lastname)

	subject := fmt.Sprintf("Warning, project %s is approaching it's enddate in 4 weeks !", pid)

	// message
	tempStr := `Dear {{.Name}},

Please be aware that project '{{.ProjectName}}' ({{.ProjectID}}) where you are a manager/contributor will expire on {{.ProjectEndDate}}. This has consequences regarding the assigned quota to this project.

If this project has finished please take care the data is securely archived, remove the remaining data in the project directory and send an email to the helpdesk@donders.ru.nl that everything is properly archived and that the project can be deleted from central storage.

More information on project expiration and quota :

  - ProjectExpirationProcedure (see https://intranet.donders.ru.nl/uploads/media/20190624-ProjectExpirationProcedure-Rev3.pdf)
  - Quota on central storage (see https://intranet.donders.ru.nl/index.php?id=quota)

In case of any questions, please send an e-mail to the Project Database Administration (Sabita Raktoe).

With kind regards,

The project administration
Room number 0.021
Phone (+3124 36) 10750
	
Sabita Raktoe
Management Assistant DCCN
`
	// data for message template
	tempData := struct {
		Name           string
		ProjectID      string
		ProjectName    string
		ProjectEndDate string
	}{name, pid, pname, penddate}

	body, err := composeMessageTempstr(tempStr, tempData)

	if err != nil {
		return err
	}

	return sendMail(m.config, from, recipient.Email, subject, body)
}

// AlertProjectStorageOoq sends out alert email concerning project (about to) running out-of-quota.
func (m *Mailer) AlertProjectStorageOoq(recipient pdb.User, storageInfo pdb.StorageInfo, pid, pname string) error {

	from := "no-reply@donders.ru.nl"
	name := fmt.Sprintf("%s %s", recipient.Firstname, recipient.Lastname)

	var uratio int
	if storageInfo.QuotaGb == 0 && storageInfo.UsageMb > 0 {
		uratio = 100
	} else {
		uratio = 100 * storageInfo.UsageMb / (storageInfo.QuotaGb << 10)
	}

	if uratio > 100 {
		uratio = 100
	}

	subject := fmt.Sprintf("Warning, storage of your project %s is %d%% full", pid, uratio)

	// message template
	tempStr := `Dear {{.Name}},

You received this warning because you are the applicant and/or a manager and/or a contributor of the project {{.ProjectID}} with title:

    {{.ProjectName}}

The quota for your project directory {{.ProjectID}} is with {{.QuotaUsageRatio}}% usage close to being full. 

Be aware that when there is no quota any more, you may encounter issues such as:

    - not automatically receiving MEG and MRI raw data (see https://intranet.donders.ru.nl/index.php?id=archiving-autotransfer)
    - not being able to use the lab uploader (see https://intranet.donders.ru.nl/index.php?id=uploader)
    - unexpected failures in data analyses and batch jobs on the cluster
    - etc.

Please consider to clean up the project directory (i.e. /project/{{.ProjectID}} or P:\{{.ProjectID}}) when possible.

If more quota is needed, please see the procedure described in the "Exceptional quota requests" section of the following intranet page: https://intranet.donders.ru.nl/index.php?id=quota

If you have further questions, don't hesitate to contact the TG helpdesk (helpdesk@fcdonders.ru.nl).

Best regards, the DCCN Technical Group
`

	// data for message template
	tempData := struct {
		Name            string
		ProjectID       string
		ProjectName     string
		QuotaUsageRatio int
	}{name, pid, pname, uratio}

	body, err := composeMessageTempstr(tempStr, tempData)

	if err != nil {
		return err
	}

	return sendMail(m.config, from, recipient.Email, subject, body)
}

// NotifyProjectProvisioned sends out email notification
// to `manager` about the just provisioned project `pid`.
func (m *Mailer) NotifyProjectProvisioned(manager pdb.User, pid string, pname string) error {

	from := "helpdesk@fcdonders.ru.nl"
	name := fmt.Sprintf("%s %s", manager.Firstname, manager.Lastname)
	subject := fmt.Sprintf("Storage of your project %s has been initalized", pid)

	// message template
	tempStr := `Dear {{.Name}},

The storage of your project {{.ProjectID}} with title

    {{.ProjectName}}

has been initialised.
	
You may now access the storage via the following paths:
	
    * on Windows desktop: P:\{{.ProjectID}}
    * in the cluster: /project/{{.ProjectID}}
	
For managing data access permission for project collaborators, please follow the guide:
	
    http://hpc.dccn.nl/docs/project_storage/access_management.html
	
For more information about the project storage, please refer to the intranet page:
	
    https://intranet.donders.ru.nl/index.php?id=4733
	
Should you have any questions, please don't hesitate to contact the TG helpdesk <helpdesk@fcdonders.ru.nl>.
	
Best regards, the DCCN Technical Group`

	// data for message template
	tempData := struct {
		Name        string
		ProjectID   string
		ProjectName string
	}{name, pid, pname}

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

// func encodeRFC2047(String string) string {
// 	// use mail's rfc2047 to encode any string
// 	addr := mail.Address{String, ""}
// 	return strings.Trim(addr.String(), " <>")
// }

// sendMail sends out a email with given `from`, `to`, `subject` and `body` content
// using the SMTP server configuration provided by `config`.
func sendMail(config config.SMTPConfiguration, from, to, subject, body string) error {

	// SMTP server address
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	header := make(map[string]string)
	header["From"] = from
	header["To"] = to
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"utf-8\""
	header["Content-Transfer-Encoding"] = "base64"

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + base64.StdEncoding.EncodeToString([]byte(body))

	// SMTP plain auth with username/password
	if config.AuthPlainUser != "" && config.AuthPlainPass != "" {
		auth := smtp.PlainAuth("", config.AuthPlainUser, config.AuthPlainPass, config.Host)
		return smtp.SendMail(addr, auth, from, []string{to}, []byte(message))
	}

	// no SMTP authentication
	return smtp.SendMail(addr, nil, from, []string{to}, []byte(message))
}
