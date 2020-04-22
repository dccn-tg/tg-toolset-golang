package config

// PDBConfiguration defines the configuration parameters for project database.
type PDBConfiguration struct {
	Version int
	V1      DBConfiguration
	V2      CoreAPIConfiguration
}

// CoreAPIConfiguration defines the configuration parameters for the core api of the project database v2.
type CoreAPIConfiguration struct {
	AuthClientSecret string `mapstructure:"auth_client_secret"`
	AuthURL          string `mapstructure:"auth_url"`
	CoreAPIURL       string `mapstructure:"core_api_url"`
}
