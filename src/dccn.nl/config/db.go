package config

// SqlConfiguration is the data structure for marshaling the
// pdb section of the config.yml file using the viper
// configuration framework.
type DBConfiguration struct {
	HostSQL     string
	PortSQL     int
	UserSQL     string
	PassSQL     string
	DatabaseSQL string
}
