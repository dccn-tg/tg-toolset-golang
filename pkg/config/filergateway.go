package config

// FilerGatewayConfiguration is the data structure for marshaling
// the client configuration for connecting to the filer-gateway
// service.
type FilerGatewayConfiguration struct {
	APIKey  string `mapstructure:"api_key"`
	APIURL  string `mapstructure:"api_url"`
	APIUser string `mapstructure:"api_user"`
	APIPass string `mapstructure:"api_pass"`
}

// NetAppCLIConfiguration is the data structure for marshaling
// the client configuration for performing management command
// on the OnTAP CLI console via SSH.
type NetAppCLIConfiguration struct {
	MgmtHost      string `mapstructure:"ssh_host"`
	ProjectVolume string `mapstructure:"vol_name_project"`
	SVM           string `mapstructure:"svm_name"`
	ExportPolicy  string `mapstructure:"export_policy"`
}
