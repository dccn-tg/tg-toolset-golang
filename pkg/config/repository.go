package config

// RepositoryConfiguration is the data structure for marshaling
// the client configuration for managing repository data.
type RepositoryConfiguration struct {
	// Username is the data-access account username of the Donders Repository
	Username string `mapstructure:"username"`
	// Password is the data-access account password of the Donderes Repository
	Password string `mapstructure:"password"`
	// BaseURL is the webdav endpoint of the root directory or a collection in the Donders Repository.
	BaseURL string `mapstructure:"baseurl,omitempty"`
	// UmapDomain needs to be a slice of pointer to get
	// viper unmarshal list in the YAML file properly.
	UmapDomains []*string `mapstructure:"umap_domains,omitempty"`
}
