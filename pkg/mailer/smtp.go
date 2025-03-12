package mailer

import (
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
)

type SMTPMailer struct {
	config config.SMTPConfiguration
}

// SendMail sends out a email with given `from`, `to`, `subject` and `body` content
// using the SMTP server configuration provided by `config`.
func (m SMTPMailer) SendMail(from, subject, body string, to []string, cc ...string) error {

	// SMTP server address
	addr := fmt.Sprintf("%s:%d", m.config.Host, m.config.Port)

	header := make(map[string]string)
	header["From"] = from
	header["To"] = strings.Join(to, ";")
	header["Cc"] = strings.Join(cc, ";")
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
	if m.config.AuthPlainUser != "" && m.config.AuthPlainPass != "" {
		auth := smtp.PlainAuth("", m.config.AuthPlainUser, m.config.AuthPlainPass, m.config.Host)
		return smtp.SendMail(addr, auth, from, append(to, cc...), []byte(message))
	}

	// no SMTP authentication
	return smtp.SendMail(addr, nil, from, append(to, cc...), []byte(message))
}
