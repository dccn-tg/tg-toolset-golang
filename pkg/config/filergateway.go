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
