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

type contentType string

const (
	plain contentType = "text/plain; charset=\"utf-8\""
	html  contentType = "text/html; charset=\"utf-8\""
)

// composeSend composes the message body with a given body `contentType` and send the mail.
func (m SMTPMailer) composeSend(from, subject, body string, contentType contentType, to []string, cc ...string) error {

	// SMTP server address
	addr := fmt.Sprintf("%s:%d", m.config.Host, m.config.Port)

	header := make(map[string]string)
	header["From"] = from
	header["To"] = strings.Join(to, ";")
	header["Cc"] = strings.Join(cc, ";")
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = string(contentType)
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

// SendMail sends out a email with given `from`, `to`, `subject` and plain-text `body` content
// using the SMTP server configuration provided by `config`.
func (m SMTPMailer) SendMail(from, subject, body string, to []string, cc ...string) error {
	return m.composeSend(
		from,
		subject,
		body,
		plain,
		to,
		cc...,
	)
}

// SendHtmlMail sends out a email with given `from`, `to`, `subject` and html-text `body` content
// using the SMTP server configuration provided by `config`.
func (m SMTPMailer) SendHtmlMail(from, subject, body string, to []string, cc ...string) error {

	return m.composeSend(
		from,
		subject,
		body,
		html,
		to,
		cc...,
	)
}
