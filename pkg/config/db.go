package config

// DBConfiguration is the data structure for marshaling the
// SQL database configuration sessions of the config.yml file
// using the viper configuration framework.
type DBConfiguration struct {
	HostSQL     string
	PortSQL     int
	UserSQL     string
	PassSQL     string
	DatabaseSQL string
}
