// Package pdb2 implements connection to the Project Database 2.0,
// using the GraphQL interface.
package pdb2

import log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"

func init() {

	cfg := log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Info,
	}

	// initialize logger
	log.NewLogger(cfg, log.InstanceLogrusLogger)
}
