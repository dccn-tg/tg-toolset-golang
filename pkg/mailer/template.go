package mailer

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"
	"strings"
)

/** An template file example:

```
Storage of your project {{.ProjectID}} has been initalized!

Dear {{.RecipientName}},

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

Should you have any questions, please don't hesitate to contact the TG helpdesk <helpdesk@donders.ru.nl>.

Best regards, {{.SenderName}}
```
**/

type ProjectAlertTemplateData struct {
	ProjectID        string // project id
	ProjectTitle     string // project title
	ProjectEndDate   string // project end date in format of "2006-01-02"
	RecipientName    string // full name of the alert recipient to be addressed
	SenderName       string // full name of the alert sender
	ExpiringInDays   int    // number of days before the project's end date
	ExpiringInMonths int    // number of months before the project's end date
	QuotaUsageRatio  int    // project storage quota usage ratio
}

// template function definition
var funcMap = template.FuncMap{
	// The name "neg" is a template function to convert integer to negtive value.
	"neg": func(i int) int {
		return 0 - i
	},
}

// ComposeMessageFromTempfile composes subject and body of a message using the `tempfile` and the `data`
// provided.
func ComposeMessageFromTemplateFile(tempfile string, data interface{}) (string, string, error) {

	var buf bytes.Buffer
	t, err := template.New(filepath.Base(tempfile)).Funcs(funcMap).ParseFiles([]string{tempfile}...)

	if err != nil {
		return "", "", err
	}

	err = t.Execute(&buf, data)
	if err != nil {
		return "", "", err
	}

	// the first non-empty line is taken as subject, and the rest
	// (excluding empty lines righ below the subject) is taken as body.
	var subject, body string
	skipEmpty := true
	scanner := bufio.NewScanner(strings.NewReader(buf.String()))
	for scanner.Scan() {

		l := strings.TrimRight(scanner.Text(), " ")

		// skip any empty lines
		if skipEmpty && len(l) == 0 {
			continue
		}

		if subject == "" {
			subject = l
		} else {
			body = fmt.Sprintf("%s%s\n", body, scanner.Text())
			skipEmpty = false
		}

	}

	if err := scanner.Err(); err != nil {
		return "", "", fmt.Errorf("fail read template for body: %s", err)
	}

	return subject, body, nil
}
