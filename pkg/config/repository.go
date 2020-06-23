package config

// RepositoryConfiguration is the data structure for marshaling
// the client configuration for managing repository data.
type RepositoryConfiguration struct {
	// UmapDomain needs to be a slice of pointer to get
	// viper unmarshal list in the YAML file properly.
	UmapDomains []*string `mapstructure:"umap_domains"`
}
