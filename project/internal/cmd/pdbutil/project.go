package pdbutil

import (
	"encoding/json"
	"fmt"
	"strconv"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/filergateway"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
	"github.com/spf13/cobra"
)

// loadNetAppCLI initialize interface to the NetAppCLI.
func loadNetAppCLI() filergateway.NetAppCLI {
	conf := loadConfig()
	return filergateway.NetAppCLI{Config: conf.NetAppCLI}
}

func init() {

	projectActionCmd.AddCommand(projectActionListCmd, projectActionExecCmd)

	projectCmd.AddCommand(projectActionCmd, projectCreateCmd)
	rootCmd.AddCommand(projectCmd)
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Utility for project",
	Long:  ``,
}

var projectCreateCmd = &cobra.Command{
	Use:   "create [projectID] [quotaGB]",
	Short: "Utility for creating project with given quota",
	Long:  ``,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		conf := loadConfig()
		cli := filergateway.NetAppCLI{Config: conf.NetAppCLI}

		iquota, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}

		data := pdb.DataProjectUpdate{
			Storage: pdb.Storage{
				System:  "netapp",
				QuotaGb: iquota,
			},
		}

		return cli.CreateProjectQtree(args[0], &data)
	},
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

		// load project database interface
		ipdb := loadPdb()

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

		// load project database interface
		ipdb := loadPdb()

		// list pending pdb actions
		log.Debugf("list pending actions")
		actions, err := ipdb.GetProjectPendingActions()
		if err != nil {
			return err
		}

		actionsOK := make(map[string]*pdb.DataProjectUpdate)

		conf := loadConfig()
		// load filer-gateway client to perform pending actions.
		// fgw, err := filergateway.NewClient(conf)
		// if err != nil {
		// 	return err
		// }

		// load netappcli interface to perform pending actions.
		cli := filergateway.NetAppCLI{Config: conf.NetAppCLI}

		// TODO: use concurrency for performing actions via the filer-gateway
		for pid, act := range actions {

			log.Debugf("executing pending action %s %+v", pid, act)
			// perform pending actions via the filer gateway; write out
			// error if failed and continue for the next project.
			// if _, err := fgw.SyncUpdateProject(pid, act, time.Second); err != nil {
			// 	log.Errorf("failure updating project %s: %s", pid, err)
			// 	continue
			// }

			if err := cli.CreateProjectQtree(pid, act); err != nil {
				log.Errorf("failure creating project %s: %s", pid, err)
				continue
			}

			// TODO: set ACL

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
