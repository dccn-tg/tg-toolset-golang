package pdbutil

import (
	"encoding/json"
	"fmt"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/filergateway"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
	"github.com/spf13/cobra"
)

func init() {

	projectActionCmd.AddCommand(projectActionListCmd, projectActionExecCmd)

	projectCmd.AddCommand(projectActionCmd)
	rootCmd.AddCommand(projectCmd)
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Utility for project",
	Long:  ``,
}

var projectActionCmd = &cobra.Command{
	Use:   "action",
	Short: "Utility for pending project actions",
	Long:  ``,
}

// subcommand to list pending project actions.
var projectActionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending project actions",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// list pending pdb actions
		log.Debugf("list pending actions")
		actions, err := ipdb.GetProjectPendingActions()
		if err != nil {
			return err
		}

		for pid, act := range actions {
			if data, err := json.Marshal(act); err != nil {
				log.Errorf("%s", err)
			} else {
				fmt.Printf("%s: %s", pid, data)
			}
		}

		return nil
	},
}

// subcommand to execute pending project actions.
var projectActionExecCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execute pending project actions",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {

		// list pending pdb actions
		log.Debugf("list pending actions")
		actions, err := ipdb.GetProjectPendingActions()
		if err != nil {
			return err
		}

		actionsOK := make(map[string]*pdb.DataProjectUpdate)

		// load configuration yml file
		conf, err := config.LoadConfig(configFile)
		if err != nil {
			return err
		}

		fgw, err := filergateway.NewClient(conf)
		if err != nil {
			return err
		}

		for pid, act := range actions {

			// initialize actionOK entry for the visited project
			if _, ok := actionsOK[pid]; !ok {
				actionsOK[pid] = &pdb.DataProjectUpdate{}
			}

			log.Debugf("executing pending action %s %+v", pid, act)
			// perform pending actions via the filer gateway; write out
			// error if failed and continue for the next project.
			if err := fgw.UpdateProject(pid, act); err != nil {
				log.Errorf("failure updating project %s: %s", pid, err)
				continue
			}

			// put successfully performed action to actionsOK map
			actionsOK[pid] = act
		}

		// clean up PDB pending actions that has been successfully performed.
		if err := ipdb.DelProjectPendingActions(actionsOK); err != nil {
			return err
		}

		return nil
	},
}
