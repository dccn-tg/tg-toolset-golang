package mailer

import (
	"bytes"
	"html/template"
)

type ProjectAlertTemplateData struct {
	ProjectID       string // project id
	ProjectTitle    string // project title
	ProjectEndDate  string // project end date in format of "2006-01-02"
	RecipientName   string // full name of the alert recipient to be addressed
	SenderName      string // full name of the alert sender
	ExpiringInDays  int    // number of days before the project's end date
	QuotaUsageRatio int    // project storage quota usage ratio
}

// template of project expiration
var projectExpiringSubject string = `Warning, project {{.ProjectID}} {{ if (eq .ExpiringInDays 0) }}expires today!{{ else }}is approaching it's enddate in {{.ExpiringInDays}} days!{{ end }}`

var projectExpiringBody string = `Dear {{.RecipientName}},
{{ if (eq .ExpiringInDays 0) }}
Please be aware that project '{{.ProjectTitle}}' ({{.ProjectID}}) where you are a manager/contributor expires today on {{.ProjectEndDate}}. This has the following consequences regarding the assigned quota to this project and the access to the project storage:

  - The project quota will be reduced to 0 from tomorrow. Data can still be accessed with only "read" and "delete" operations. It is the beginning of the 60-day grace period in which the managers/contributors should archive data to, for example, the Donders Repository.
{{ else }}
Please be aware that project '{{.ProjectTitle}}' ({{.ProjectID}}) where you are a manager/contributor will expire on {{.ProjectEndDate}}. This has the following consequences regarding the assigned quota to this project and the access to the project storage:

  - one day after the expiration, the project quota is reduced to 0. Data can still be accessed with only "read" and "delete" operations. It is the beginning of the 60-day grace period in which the managers/contributors should archive data to, for example, the Donders Repository.
{{ end }}
  - two months after the project expiration, the access to the project storage is "removed" from the users.

If this project has finished please take care the data is securely archived, remove the remaining data in the project directory and send an email to the helpdesk@donders.ru.nl that everything is properly archived and that the project can be deleted from central storage.

More information on project expiration and quota :

  - ProjectExpirationProcedure (see https://intranet.donders.ru.nl/uploads/media/20190624-ProjectExpirationProcedure-Rev3.pdf)
  - Quota on central storage (see https://intranet.donders.ru.nl/index.php?id=quota)

In case of any questions, please send an e-mail to the Project Database Administration (Sabita Raktoe).

With kind regards,

The project administration
Room number 0.021
Phone (+3124 36) 10750
	
{{.SenderName}}
Management Assistant DCCN
`

var projectExpiredSubject string = `Warning, project {{.ProjectID}} has expired {{neg .ExpiringInDays}} days ago!`

var projectExpiredBody string = `Dear {{.RecipientName}},
{{ if (eq .ExpiringInDays -30) }}
Please be aware that project '{{.ProjectTitle}}' ({{.ProjectID}}) where you are a manager/contributor has expired {{neg .ExpiringInDays}} days ago on {{.ProjectEndDate}}.

Data in the project storage can still be accessed with only "read" and "delete" operations. In 30 days, data access to the project storage will be removed completely.

Please take care the data is securely archived, remove the remaining data in the project directory and send an email to the helpdesk@donders.ru.nl that everything is properly archived and that the project can be deleted from central storage.
{{ else }}
Please be aware that project '{{.ProjectTitle}}' ({{.ProjectID}}) where you are a manager/contributor has expired {{neg .ExpiringInDays}} days ago on {{.ProjectEndDate}}.

Data access to the project storage is going to be removed.
{{ end }}
More information on project expiration and quota :

  - ProjectExpirationProcedure (see https://intranet.donders.ru.nl/uploads/media/20190624-ProjectExpirationProcedure-Rev3.pdf)
  - Quota on central storage (see https://intranet.donders.ru.nl/index.php?id=quota)

In case of any questions, please send an e-mail to the Project Database Administration (Sabita Raktoe).

With kind regards,

The project administration
Room number 0.021
Phone (+3124 36) 10750
	
{{.SenderName}}
Management Assistant DCCN
`

// template for project running out of quota
var projectOutOfQuotaSubject string = `Warning, storage of your project {{.ProjectID}} is {{.QuotaUsageRatio}}% full`

var projectOutOfQuotaBody string = `Dear {{.RecipientName}},

You received this warning because you are the applicant and/or a manager and/or a contributor of the project {{.ProjectID}} with title:

    {{.ProjectTitle}}

The quota for your project directory {{.ProjectID}} is with {{.QuotaUsageRatio}}% usage close to being full. 

Be aware that when there is no quota any more, you may encounter issues such as:

    - not automatically receiving MEG and MRI raw data (see https://intranet.donders.ru.nl/index.php?id=archiving-autotransfer)
    - not being able to use the lab uploader (see https://intranet.donders.ru.nl/index.php?id=uploader)
    - unexpected failures in data analyses and batch jobs on the cluster
    - etc.

Please consider to clean up the project directory (i.e. /project/{{.ProjectID}} or P:\{{.ProjectID}}) when possible.

If more quota is needed, please see the procedure described in the "Exceptional quota requests" section of the following intranet page: https://intranet.donders.ru.nl/index.php?id=quota

If you have further questions, don't hesitate to contact the TG helpdesk (helpdesk@fcdonders.ru.nl).

Best regards, {{.SenderName}}
`

// template for project being provisioned.
var projectProvisionedSubject string = `Storage of your project {{.ProjectID}} has been initalized!`

var projectProvisionedBody string = `Dear {{.RecipientName}},

The storage of your project {{.ProjectID}} with title

    {{.ProjectTitle}}

has been initialised.

You may now access the storage via the following paths:

    * on Windows desktop: P:\{{.ProjectID}}
    * in the cluster: /project/{{.ProjectID}}

For managing data access permission for project collaborators, please follow the guide:

    http://hpc.dccn.nl/docs/project_storage/access_management.html

For more information about the project storage, please refer to the intranet page:

    https://intranet.donders.ru.nl/index.php?id=4733

Should you have any questions, please don't hesitate to contact the TG helpdesk <helpdesk@fcdonders.ru.nl>.

Best regards, {{.SenderName}}
`

// ComposeProjectExpiringAlert composes the subject and body of the email alert concerning
// project approaching its end date.
func ComposeProjectExpiringAlert(data ProjectAlertTemplateData) (string, string, error) {
	subject, err := composeMessageTempstr(projectExpiringSubject, data)
	if err != nil {
		return "", "", err
	}
	body, err := composeMessageTempstr(projectExpiringBody, data)
	if err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// ComposeProjectExpiredAlert composes the subject and body of the email alert concerning
// project has passed its end date.
func ComposeProjectExpiredAlert(data ProjectAlertTemplateData) (string, string, error) {
	subject, err := composeMessageTempstr(projectExpiredSubject, data)
	if err != nil {
		return "", "", err
	}
	body, err := composeMessageTempstr(projectExpiredBody, data)
	if err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// ComposeProjectOutOfQuotaAlert composes the subject and body of the email alert concerning
// project has running out or close to its quota limitation.
func ComposeProjectOutOfQuotaAlert(data ProjectAlertTemplateData) (string, string, error) {
	subject, err := composeMessageTempstr(projectOutOfQuotaSubject, data)
	if err != nil {
		return "", "", err
	}
	body, err := composeMessageTempstr(projectOutOfQuotaBody, data)
	if err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// ComposeProjectProvisionedAlert composes the subject and body of the email alert concerning
// project has been provisioned on the storage.
func ComposeProjectProvisionedAlert(data ProjectAlertTemplateData) (string, string, error) {
	subject, err := composeMessageTempstr(projectProvisionedSubject, data)
	if err != nil {
		return "", "", err
	}
	body, err := composeMessageTempstr(projectProvisionedBody, data)
	if err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// template function definition
var funcMap = template.FuncMap{
	// The name "neg" is a template function to convert integer to negtive value.
	"neg": func(i int) int {
		return 0 - i
	},
}

// composeMessage composes a message using the given `tempfile` template file and the `data`
// provided.
func composeMessageTempfile(tempfile string, data interface{}) (string, error) {
	var buf bytes.Buffer
	t := template.Must(template.New("message").Funcs(funcMap).ParseFiles([]string{tempfile}...))
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
	t := template.Must(template.New("message").Funcs(funcMap).Parse(tempstr))
	err := t.Execute(&buf, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
