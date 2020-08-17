package pdbutil

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/Donders-Institute/tg-toolset-golang/pkg/mailer"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/filergateway"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
	"github.com/spf13/cobra"
)

var (
	execNthreads int
	storSystem   string
	useNetappCLI bool
	projectRoots map[string]string = map[string]string{
		"netapp":  "/project",
		"freenas": "/project_freenas",
		"cephfs":  "/project_cephfs",
	}
)

// loadNetAppCLI initialize interface to the NetAppCLI.
func loadNetAppCLI() filergateway.NetAppCLI {
	conf := loadConfig()
	return filergateway.NetAppCLI{Config: conf.NetAppCLI}
}

func init() {

	// get supported storage systems from the `projectRoots`.
	supportedStorSystems := make([]string, len(projectRoots))
	i := 0
	for sys := range projectRoots {
		supportedStorSystems[i] = sys
		i++
	}

	projectActionExecCmd.Flags().IntVarP(&execNthreads, "nthreads", "n", 4,
		"`number` of concurrent worker threads.")
	projectActionCmd.AddCommand(projectActionListCmd, projectActionExecCmd)

	projectCmd.PersistentFlags().StringVarP(&storSystem, "sys", "s", "netapp",
		fmt.Sprintf("storage `system`.  Supported systems: %s", strings.Join(supportedStorSystems, ",")))

	projectCmd.PersistentFlags().BoolVarP(&useNetappCLI, "netapp-cli", "", false,
		"use NetApp ONTAP CLI to apply changes on the NetApp filer. Only applicable for the netapp storage system.")

	projectCmd.AddCommand(projectActionCmd, projectCreateCmd)

	rootCmd.AddCommand(projectCmd)
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Utility for managing project storage",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if _, ok := projectRoots[storSystem]; !ok {
			log.Fatalf("unsupported storage system: %s", storSystem)
		}
		rootCmd.PersistentPreRun(cmd, args)
	},
}

var projectCreateCmd = &cobra.Command{
	Use:   "create [projectID] [quotaGB]",
	Short: "Creates or updates project storage with given quota",
	Long:  ``,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		iquota, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}

		data := pdb.DataProjectUpdate{
			Storage: pdb.Storage{
				System:  storSystem,
				QuotaGb: iquota,
			},
		}

		return actionExec(args[0], &data)
	},
}

var projectActionCmd = &cobra.Command{
	Use:   "action",
	Short: "Utility for managing pending project storage actions",
	Long:  ``,
}

// subcommand to list pending project actions.
var projectActionListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists pending project storage actions",
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
				fmt.Printf("%s: %s\n", pid, data)
			}
		}

		return nil
	},
}

// subcommand to execute pending project actions.
var projectActionExecCmd = &cobra.Command{
	Use:   "exec",
	Short: "Executes pending project storage actions",
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
		var wg sync.WaitGroup
		pids := make(chan string, execNthreads*2)
		for w := 0; w < execNthreads; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
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

		// wait for all workers to finish
		wg.Wait()

		return nil
	},
}

// actionExec implements the logic of executing the pending actions concerning a project.
func actionExec(pid string, act *pdb.DataProjectUpdate) error {

	// load project database interface
	ipdb := loadPdb()
	conf := loadConfig()

	log.Debugf("[%s] pending actions: %+v", pid, act)

	// check if the action concerns creation of a new project.
	// TODO: the project directory prefix should depends on storSystem
	ppath := filepath.Join("/project", pid)
	_, err := os.Stat(ppath)
	newProject := !os.IsNotExist(err)

	// extract member roles from the `act`
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

	if useNetappCLI && storSystem == "netapp" {
		// use NetappCLI + SSH to perform pending actions.
		cli := filergateway.NetAppCLI{Config: conf.NetAppCLI}
		if !newProject {
			log.Infof("[%s] project path already exists: %s", pid, ppath)
			// try update the project quota
			if err := cli.UpdateProjectQuota(pid, act); err != nil {
				return fmt.Errorf("[%s] fail updating project quota: %s", pid, err)
			}
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

	} else {
		// use filer-gateway client to perform pending actions.
		fgw, err := filergateway.NewClient(conf)
		if err != nil {
			return err
		}

		// perform pending actions via the filer gateway; write out
		// error if failed and continue for the next project.
		if _, err := fgw.SyncUpdateProject(pid, act, time.Second); err != nil {
			return fmt.Errorf("[%s] failure updating project: %s", pid, err)
		}
	}

	// make sure the directory of the created project is owned by `project:project_g`
	if newProject {
		projectOwner := "project"
		u, err := user.Lookup(projectOwner)
		if err != nil {
			log.Errorf("[%s] cannot find system user %s: %s", pid, projectOwner, err)
		}
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)
		if err := os.Chown(ppath, uid, gid); err != nil {
			log.Errorf("[%s] cannot change owner of %s: %s", pid, ppath, err)
		}
	}

	// For PDBv1, get ACL from the project and update the active members into the database.
	if v1, ok := ipdb.(pdb.V1); ok {

		runner := acl.Runner{
			RootPath:   ppath,
			FollowLink: false,
		}

		// get roles associated with the project directory, do not iterate over files/sub-directories.
		rolePathMap, err := runner.GetRoles(false)
		if err != nil {
			return fmt.Errorf("[%s] fail getting acl: %s", pid, err)
		}

		// construct data structure for updating PDB v1 database.
		members := []pdb.Member{}
		for rolePath := range rolePathMap {
			for r, uids := range rolePath.RoleMap {
				for _, u := range uids {
					members = append(members, pdb.Member{
						Role:   r.String(),
						UserID: u,
					})
				}
			}
		}

		// update PDB v1 database with the up-to-date active members.
		if err := v1.UpdateProjectMembers(pid, members); err != nil {
			return fmt.Errorf("[%s] fail updating acl in PDB: %s", pid, err)
		}
	}

	// put successfully performed action to actionsOK map
	// initialize map for successfully performed actions.
	actionsOK := map[string]*pdb.DataProjectUpdate{
		pid: act,
	}

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
