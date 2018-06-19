package config

// Configuration is the data structure for marshaling the
// config.yml file using the viper configuration framework.
type Configuration struct {
	PDB DBConfiguration
	CDB DBConfiguration
}
