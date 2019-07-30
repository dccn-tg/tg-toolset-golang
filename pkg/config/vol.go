package config

// VolumeManagerConfiguration is the data structure for marshaling the
// volume manager configuration sessions of the config.yml file
// using the viper configuration framework.
type VolumeManagerConfiguration struct {
	// ManagementInterface configures the management interfaces of various storage servers.
	ManagementInterface ManagementInterfaceConfig
}

// ManagementInterfaceConfig is the data structure for marshaling the
// management interface configuration of various storage servers.
type ManagementInterfaceConfig struct {
	NetApp  string
	FreeNas string
}
