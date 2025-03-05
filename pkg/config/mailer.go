package config

type MailerConfiguration struct {
	SMTP  SMTPConfiguration
	Graph GraphConfiguration
}

// SMTPConfiguration is the data structure for marshaling the
// SMTP server configuration sessions of the config.yml file
// using the viper configuration framework.
type SMTPConfiguration struct {
	Host          string `mapstructure:"host"`
	Port          int    `mapstructure:"port"`
	AuthPlainUser string `mapstructure:"auth_plain_user"`
	AuthPlainPass string `mapstructure:"auth_plain_pass"`
}

type GraphConfiguration struct {
	TenantID                string `mapstructure:"tenant_id"`
	ApplicationID           string `mapstructure:"application_id"`
	ClientSecret            string `mapstructure:"client_secret"`
	ClientCertificate       string `mapstructure:"client_certificate"`
	ClientCertificatePass   string `mapstructure:"client_certificate_pass"`
	SenderUserPrincipalName string `mapstructure:"sender_upn"`
}
