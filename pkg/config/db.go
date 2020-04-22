package config

// DBConfiguration is the data structure for marshaling the
// SQL database configuration sessions of the config.yml file
// using the viper configuration framework.
type DBConfiguration struct {
	HostSQL     string `mapstructure:"db_host"`
	PortSQL     int    `mapstructure:"db_port"`
	UserSQL     string `mapstructure:"db_user"`
	PassSQL     string `mapstructure:"db_pass"`
	DatabaseSQL string `mapstructure:"db_name"`
}
