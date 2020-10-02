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
	"github.com/Donders-Institute/tg-toolset-golang/pkg/store"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/filergateway"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
	"github.com/spf13/cobra"
)

var (
	execNthreads      int
	storSystem        string
	useNetappCLI      bool
	activeProjectOnly bool
	projectRoots      map[string]string = map[string]string{
		"netapp":  "/project",
		"freenas": "/project_freenas",
		"cephfs":  "/project_cephfs",
	}
	ooqNotificationDbPath string
)

// loadNetAppCLI initialize interface to the NetAppCLI.
func loadNetAppCLI() filergateway.NetAppCLI {
	conf := loadConfig()
	return filergateway.NetAppCLI{Config: conf.NetAppCLI}
}

// ooqNotificationFrequency determines the max ooq notification frequency
// in terms of the duration between two subsequent notifications, based on
// the current storage usage in percentage.
//
// if the return duration is 0, it means "never".
//
// TODO: make the usage range and the frequency configurable
func ooqNotificationFrequency(usage int) time.Duration {

	if usage >= 90 && usage < 95 {
		return time.Hour * 24 * 14 // every 14 days
	}

	if usage >= 95 && usage < 99 {
		return time.Hour * 24 * 7 // every 7 days
	}

	if usage >= 99 {
		return time.Hour * 24 * 2 // every 2 days
	}

	return 0
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

	projectUpdateMembersCmd.Flags().BoolVarP(&activeProjectOnly, "active-only", "a", false,
		"only update members on the active projects")

	projectUpdateCmd.AddCommand(projectUpdateMembersCmd)

	projectNotifyOutOfQuota.Flags().StringVarP(&ooqNotificationDbPath, "dbpath", "", "ooq-notification.db",
		"`path` of the out-of-quota notification history")

	projectNotifyCmd.AddCommand(projectNotifyOutOfQuota)

	projectCmd.AddCommand(projectActionCmd, projectCreateCmd, projectUpdateCmd, projectNotifyCmd)

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

var projectUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Updates project attributes in the project database",
	Long:  ``,
}

var projectUpdateMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "Updates project members with access roles in the project storage",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ipdb := loadPdb()
		conf := loadConfig()

		pids, err := ipdb.GetProjects(true)
		if err != nil {
			return err
		}

		log.Debugf("updating members for %d projects", len(pids))

		// initialize filergateway client
		fgw, err := filergateway.NewClient(conf)
		if err != nil {
			return err
		}

		// perform pending actions with 4 concurrent workers,
		// each works on a project.
		var wg sync.WaitGroup
		cpids := make(chan string, execNthreads*2)
		for w := 0; w < execNthreads; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for pid := range cpids {
					// get members from filer gateway
					info, err := fgw.GetProject(pid)
					if err != nil {
						log.Errorf("[%s] cannot get project storage info: %s", pid, err)
						continue
					}
					log.Debugf("[%s] project storage info: %+v", pid, info)

					// update project database only for pdb V1.
					if v1, ok := ipdb.(pdb.V1); ok {
						if err := v1.UpdateProjectMembers(pid, info.Members); err != nil {
							log.Errorf("[%s] cannot update members in pdb: %s", pid, err)
						}
					} else {
						log.Warnf("[%s] not pdb V1: skip updating members.")
					}
				}
			}()
		}

		for _, pid := range pids {
			cpids <- pid
		}
		close(cpids)

		// wait for all workers to finish
		wg.Wait()

		return nil
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

var projectNotifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Utility for sending project-related notification",
	Long:  ``,
}

// submcommand to notify manager/contributor/owner when project is (close to) running out of quota.
var projectNotifyOutOfQuota = &cobra.Command{
	Use:   "ooq",
	Short: "Sends notification concerning project running out of quota",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {

		ipdb := loadPdb()
		conf := loadConfig()

		pids, err := ipdb.GetProjects(true)
		if err != nil {
			return err
		}

		log.Debugf("retriving storage usage for %d projects", len(pids))

		// initialize filergateway client
		fgw, err := filergateway.NewClient(conf)
		if err != nil {
			return err
		}

		// connect to internal database for last sent
		store := store.KVStore{
			Path: ooqNotificationDbPath,
		}
		err = store.Connect()
		if err != nil {
			return err
		}
		defer store.Disconnect()

		// initialize kvstore with bucket "ooqLastNotifications"
		dbBucket := "ooqLastNotifications"
		err = store.Init([]string{dbBucket})
		if err != nil {
			return err
		}

		// perform pending actions with 4 concurrent workers,
		// each works on a project.
		var wg sync.WaitGroup
		cpids := make(chan string, execNthreads*2)
		for w := 0; w < execNthreads; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				for pid := range cpids {
					// get members from filer gateway
					info, err := fgw.GetProject(pid)
					if err != nil {
						log.Errorf("[%s] cannot get project storage info: %s", pid, err)
						continue
					}
					log.Debugf("[%s] project storage info: %+v", pid, info)

					// get last sent timestamp from the local db
					tsd, err := store.Get(dbBucket, []byte(pid))
					if err != nil {
						log.Debugf("[%s] fail to get last ooq notification time: %s", pid, err)
					}

					// default ts of the last sent (0001-01-01 00:00:00 +0000 UTC)
					ts := time.Time{}
					if tsd != nil {
						if err := json.Unmarshal(tsd, &ts); err != nil {
							log.Debugf("[%s] cannot interpret last ooq notification time: %s", pid, err)
						}
					}
					log.Debugf("[%s] last ooq notifiction time: %s", pid, ts)

					// check and send notification
					switch npid, err := notifyOoq(ipdb, info, &ts); err.(type) {
					case nil:
						// notification sent, update store db with new last sent timestamp
						tsb, _ := json.Marshal(ts)
						store.Set(dbBucket, []byte(npid), tsb)
					case *pdb.OpsIgnored:
						// notification ignored
						log.Debugf("[%s] %s", pid, err)
					default:
						// something wrong
						log.Errorf("[%s] fail to send notification for project out-of-quota: +%v", pid, err)
					}
				}
			}()
		}

		for _, pid := range pids {
			cpids <- pid
		}
		close(cpids)

		// wait for all workers to finish
		wg.Wait()

		return nil
	},
}

// notifyOoq checks whether notification concerning project storage out-of-quota
// is to be sent based on the project storage information `info`.
//
// If the notification email is sent, the returning string is the concerning project id;
// otherwise an empty string is returned with an error.
//
// If the notification sending is ignored by design, the returned error is `OpsIgnored`.
func notifyOoq(ipdb pdb.PDB, info *pdb.DataProjectInfo, lastSent *time.Time) (string, error) {

	uratio := 100 * info.Storage.UsageGb / info.Storage.QuotaGb

	duration := ooqNotificationFrequency(uratio)
	if duration == 0 {
		return "", &pdb.OpsIgnored{Message: fmt.Sprintf("usage (%d%%) below the ooq threshold.", uratio)}
	}

	now := time.Now()
	next := lastSent.Add(duration)
	if now.Before(next) { // current time is in between
		return "", &pdb.OpsIgnored{Message: fmt.Sprintf("%s not reaching next notification %s.", now, next)}
	}

	// sending notifications
	for _, m := range info.Members {
		if m.Role == acl.Manager.String() || m.Role == acl.Contributor.String() {
			u, err := ipdb.GetUser(m.UserID)
			if err != nil {
				log.Errorf("[%s] cannot get user from project database: %s", m.UserID)
				continue
			}
			log.Debugf("[%s] notify %s on usage ratio: %d", info.ProjectID, u.Email, uratio)
			// TODO: implement email sending
		}
	}

	// set lastSent to now
	lastSent = &now

	return info.ProjectID, nil
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
	newProject := os.IsNotExist(err)

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
				// timeout after 5 minutes
				if time.Since(t1) > 5*time.Minute {
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

		t1 := time.Now()
		// check until the project directory aappears
		for {
			if _, err := os.Stat(ppath); !os.IsNotExist(err) {
				break
			}
			// timeout after 5 minutes
			if time.Since(t1) > 5*time.Minute {
				log.Errorf("[%s] timeout waiting for %s to appear", pid, ppath)
				break
			}
			// wait for 100 millisecond for the next check.
			time.Sleep(100 * time.Millisecond)
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
