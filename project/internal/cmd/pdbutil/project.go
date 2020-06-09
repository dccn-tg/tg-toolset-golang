package pdbutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/Donders-Institute/tg-toolset-golang/pkg/mailer"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"
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

		// list pending pdb actions
		log.Debugf("list pending actions")
		actions, err := loadPdb().GetProjectPendingActions()
		if err != nil {
			return err
		}

		// perform pending actions with 4 concurrent workers,
		// each works on a project.
		nworkers := 4
		pids := make(chan string, nworkers*2)
		for w := 1; w <= nworkers; w++ {
			go func() {
				for pid := range pids {
					if err := actionExec(pid, actions[pid]); err != nil {
						log.Errorf("%s", err)
					}
				}
			}()
		}

		for pid := range actions {
			pids <- pid
		}
		close(pids)

		return nil
	},
}

// actionExec implements the logic of executing the pending actions concerning a project.
func actionExec(pid string, act *pdb.DataProjectUpdate) error {

	// load project database interface
	ipdb := loadPdb()
	conf := loadConfig()

	// METHOD1: use filer-gateway client to perform pending actions.
	// fgw, err := filergateway.NewClient(conf)
	// if err != nil {
	// 	return err
	// }
	//
	// perform pending actions via the filer gateway; write out
	// error if failed and continue for the next project.
	// if _, err := fgw.SyncUpdateProject(pid, act, time.Second); err != nil {
	// 	return fmt.Errorf("failure updating project %s: %s", pid, err)
	// }

	// METHOD2: use netappcli interface to perform pending actions.
	cli := filergateway.NetAppCLI{Config: conf.NetAppCLI}

	actionsOK := make(map[string]*pdb.DataProjectUpdate)

	log.Debugf("[%s] pending actions: %+v", pid, act)

	ppath := filepath.Join("/project", pid)

	_, err := os.Stat(ppath)
	newProject := !os.IsNotExist(err)

	if !newProject {
		log.Infof("[%s] project path already exists: %s", pid, ppath)
		// // try update the project quota
		// if err := cli.UpdateProjectQuota(pid, act); err != nil {
		// 	return fmt.Errorf("[%s] fail updating project quota: %s", pid, err)
		// }
	} else {
		log.Infof("[%s] creating new project on path: %s", pid, ppath)
		if err := cli.CreateProjectQtree(pid, act); err != nil {
			return fmt.Errorf("[%s] fail creating project: %s", pid, err)
		}

		t1 := time.Now()
		// check until the project directory aappears
		for {
			if _, err := os.Stat(ppath); !os.IsNotExist(err) {
				break
			}
			// timeout after 3 minutes
			if time.Since(t1) > 3*time.Minute {
				log.Errorf("[%s] timeout waiting for %s to appear", pid, ppath)
				break
			}
			// wait for 100 millisecond for the next check.
			time.Sleep(100 * time.Millisecond)
		}

		// move to next project if the ppath still doesn't exist.
		if _, err := os.Stat(ppath); os.IsNotExist(err) {
			return fmt.Errorf("[%s] project path not found: %s", pid, ppath)
		}
	}

	// apply ACL setting based on the given project member roles.
	managers := []string{}
	contributors := []string{}
	viewers := []string{}
	removal := []string{}

	for _, m := range act.Members {
		switch m.Role {
		case acl.Manager.String():
			managers = append(managers, m.UserID)
		case acl.Contributor.String():
			contributors = append(contributors, m.UserID)
		case acl.Viewer.String():
			viewers = append(viewers, m.UserID)
		case "none":
			removal = append(removal, m.UserID)
		}
	}

	// set members to the project
	runner := acl.Runner{
		RootPath:     ppath,
		Managers:     strings.Join(managers, ","),
		Contributors: strings.Join(contributors, ","),
		Viewers:      strings.Join(viewers, ","),
		FollowLink:   false,
		SkipFiles:    false,
		Nthreads:     4,
		Silence:      false,
		Traverse:     false,
		Force:        false,
	}

	if ec, err := runner.SetRoles(); err != nil {
		return fmt.Errorf("[%s] fail setting member role (ec=%d): %s", pid, ec, err)
	}

	// remove members from project (only meaningful for existing project)
	if !newProject {
		runner = acl.Runner{
			RootPath:     ppath,
			Managers:     strings.Join(removal, ","),
			Contributors: strings.Join(removal, ","),
			Viewers:      strings.Join(removal, ","),
			FollowLink:   false,
			SkipFiles:    false,
			Nthreads:     4,
			Silence:      false,
			Traverse:     false,
			Force:        false,
		}

		if ec, err := runner.RemoveRoles(); err != nil {
			return fmt.Errorf("[%s] fail removing acl (ec=%d): %s", pid, ec, err)
		}
	}
	// put successfully performed action to actionsOK map
	actionsOK[pid] = act

	// clean up PDB pending actions that has been successfully performed.
	// Note that it doesn't fail the process.
	if err := ipdb.DelProjectPendingActions(actionsOK); err != nil {
		log.Errorf("[%s] fail cleaning up pending actions: %s", pid, err)
	}

	// sendout email notifying managers the new project storage is ready to use.
	if newProject {
		mailer := mailer.New(conf.SMTP)
		for _, m := range managers {

			log.Debugf("[%s] sending notification to manager %s", pid, m)

			u, err := ipdb.GetUser(m)

			if err != nil {
				log.Errorf("[%s] fail getting user profile of manager %s: %s", pid, m, err)
				continue
			}

			if err := mailer.NotifyProjectProvisioned(*u, pid); err != nil {
				log.Errorf("[%s] fail notifying manager %s: %s", pid, m, err)
			}
		}
	}

	return nil
}
