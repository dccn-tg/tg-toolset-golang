package config

// OrthancConfiguration is the data structure for marshaling the
// Orthanc PACS server configuration sessions of the config.yml file
// using the viper configuration framework.
type OrthancConfiguration struct {
	PrefixURL string
	Username  string
	Password  string
}
