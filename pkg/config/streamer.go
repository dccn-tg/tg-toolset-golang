package config

// StreamerConfiguration is the data structure for marshaling the
// Streamer server configuration sessions of the config.yml file
// using the viper configuration framework.
type StreamerConfiguration struct {
	PrefixURL string
	Username  string
	Password  string
}
