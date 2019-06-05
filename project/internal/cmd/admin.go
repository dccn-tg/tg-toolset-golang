package cmd

import "github.com/spf13/cobra"

var configFile string

func init() {
	adminCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.xml", "path of the configuration YAML file.")
	rootCmd.AddCommand(adminCmd)
}

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "project administration CLI",
	Long:  ``,
}
