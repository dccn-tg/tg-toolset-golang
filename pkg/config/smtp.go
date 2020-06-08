package config

// SMTPConfiguration is the data structure for marshaling the
// SMTP server configuration sessions of the config.yml file
// using the viper configuration framework.
type SMTPConfiguration struct {
	Host          string `mapstructure:"host"`
	Port          int    `mapstructure:"port"`
	AuthPlainUser string `mapstructure:"auth_plain_user"`
	AuthPlainPass string `mapstructure:"auth_plain_pass"`
}
