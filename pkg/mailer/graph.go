package mailer

import (
	"context"
	"fmt"
	"os"
	"time"

	azidentity "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
	log "github.com/dccn-tg/tg-toolset-golang/pkg/logger"
	msgraph "github.com/microsoftgraph/msgraph-sdk-go"
	graphmodels "github.com/microsoftgraph/msgraph-sdk-go/models"
	graphusers "github.com/microsoftgraph/msgraph-sdk-go/users"
)

type GraphMailer struct {
	config config.GraphConfiguration
}

// initClientWithCert initialize the msgraph service client using certificate.
func (m GraphMailer) initClientWithCert() (*msgraph.GraphServiceClient, error) {

	// Load certificate
	certFile, err := os.Open(m.config.ClientCertificate)
	if err != nil {
		return nil, err
	}

	info, err := certFile.Stat()
	if err != nil {
		return nil, err
	}

	certBytes := make([]byte, info.Size())
	certFile.Read(certBytes)
	certFile.Close()

	certs, key, err := azidentity.ParseCertificates(certBytes, []byte(m.config.ClientCertificatePass))
	if err != nil {
		return nil, err
	}

	cred, err := azidentity.NewClientCertificateCredential(
		m.config.TenantID,
		m.config.ApplicationID,
		certs,
		key,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return msgraph.NewGraphServiceClientWithCredentials(
		cred,
		[]string{"https://graph.microsoft.com/.default"},
	)
}

// initClientWithSecret initialize the msgraph service client using client secret.
func (m GraphMailer) initClientWithSecret() (*msgraph.GraphServiceClient, error) {

	cred, err := azidentity.NewClientSecretCredential(
		m.config.TenantID,
		m.config.ApplicationID,
		m.config.ClientSecret,
		nil,
	)

	if err != nil {
		return nil, err
	}

	return msgraph.NewGraphServiceClientWithCredentials(
		cred,
		[]string{"https://graph.microsoft.com/.default"},
	)
}

// composeMessage construct the msgraph send mail request body
func (m GraphMailer) composeMessage(
	from, subject, body string,
	to, cc []string,
) *graphusers.ItemSendMailPostRequestBody {

	requestBody := graphusers.NewItemSendMailPostRequestBody()
	message := graphmodels.NewMessage()
	message.SetSubject(&subject)

	// message body
	itemBody := graphmodels.NewItemBody()
	contentType := graphmodels.TEXT_BODYTYPE
	itemBody.SetContentType(&contentType)
	itemBody.SetContent(&body)
	message.SetBody(itemBody)

	// from address
	fromEmail := graphmodels.NewEmailAddress()
	fromEmail.SetAddress(&from)
	fromRecipient := graphmodels.NewRecipient()
	fromRecipient.SetEmailAddress(fromEmail)
	message.SetFrom(fromRecipient)

	// to recipients
	toRecipients := []graphmodels.Recipientable{}
	for _, toAddress := range to {
		recipient := graphmodels.NewRecipient()
		email := graphmodels.NewEmailAddress()
		email.SetAddress(&toAddress)
		recipient.SetEmailAddress(email)

		toRecipients = append(toRecipients, recipient)
	}
	message.SetToRecipients(toRecipients)

	// cc recipients
	ccRecipients := []graphmodels.Recipientable{}
	for _, ccAddress := range cc {
		recipient := graphmodels.NewRecipient()
		email := graphmodels.NewEmailAddress()
		email.SetAddress(&ccAddress)
		recipient.SetEmailAddress(email)

		ccRecipients = append(toRecipients, recipient)
	}
	message.SetCcRecipients(ccRecipients)

	requestBody.SetMessage(message)
	saveToSentItems := false
	requestBody.SetSaveToSentItems(&saveToSentItems)

	return requestBody
}

// SentMail sends out an email via the MS-Graph interface using the client credential.
func (m GraphMailer) SendMail(from, subject, body string, to []string, cc ...string) error {

	client, err := m.initClientWithCert()
	if err != nil {
		log.Errorf("fail to initialize graph client with certificate: %s, trying with client secret", err)
		client, err = m.initClientWithSecret()
		if err != nil {
			return fmt.Errorf("cannot initialize graph client")
		}
	}

	// compose message
	requestBody := m.composeMessage(
		from,
		subject,
		body,
		to,
		cc,
	)

	// sendmail with 1 minute timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	return client.Users().ByUserId(from).SendMail().Post(
		ctx,
		requestBody,
		nil,
	)
}
