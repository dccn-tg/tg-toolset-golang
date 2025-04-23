// Package mailer implements email notifications using the SMTP server.
package mailer

import (
	"fmt"

	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
)

type MailerProtocol int

const (
	SMTP MailerProtocol = iota
	Graph
)

// New returns a new mailer instance.
func New(config config.MailerConfiguration, protocol MailerProtocol) (Mailer, error) {
	switch protocol {
	case SMTP:
		return SMTPMailer{config: config.SMTP}, nil
	case Graph:
		return GraphMailer{config: config.Graph}, nil
	default:
		return nil, fmt.Errorf("unknown protocol: %+v", protocol)
	}
}

// Mailer implements varias email notifications.
type Mailer interface {
	SendMail(from, subject, body string, to []string, cc ...string) error
	SendHtmlMail(from, subject, body string, to []string, cc ...string) error
}
