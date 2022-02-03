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

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
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
	alertDbPath           string
	ooqAlertTestProjectID string
	ooqAlertSkipPI        bool
)

// loadNetAppCLI initialize interface to the NetAppCLI.
func loadNetAppCLI() filergateway.NetAppCLI {
	conf := loadConfig()
	return filergateway.NetAppCLI{Config: conf.NetAppCLI}
}

// ooqAlertFrequency determines the max ooq alert frequency
// in terms of the duration between two subsequent alerts, based on
// the current storage usage in percentage.
//
// if the return duration is 0, it means "never".
//
// TODO: make the usage range and the frequency configurable
func ooqAlertFrequency(usage int) time.Duration {

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

	projectUpdateCmd.Flags().BoolVarP(&activeProjectOnly, "active-only", "a", false,
		"update only the active projects")

	projectAlertOoqCmd.PersistentFlags().StringVarP(&alertDbPath, "dbpath", "", "alert.db",
		"`path` of the internal alert history database")

	projectAlertOoqSend.Flags().StringVarP(&ooqAlertTestProjectID, "test", "t", "",
		"`id` of project for testing ooq alert")

	projectAlertOoqSend.Flags().BoolVarP(&ooqAlertSkipPI, "skip-pi", "", false,
		"set to skip sending alert to PIs")

	projectAlertOoqCmd.AddCommand(projectAlertOoqInfo, projectAlertOoqSend)

	projectAlertCmd.AddCommand(projectAlertOoqCmd)

	projectCmd.AddCommand(projectActionCmd, projectCreateCmd, projectUpdateCmd, projectAlertCmd)

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

// var projectUpdateCmd = &cobra.Command{
// 	Use:   "update",
// 	Short: "Updates project attributes in the project database",
// 	Long:  ``,
// }

// temporary function to combine checks on multiple `cobra.PositionalArgs`.
// NOTE: this will be replaced by an official cobra function `MatchAll` in the future.
//       see https://github.com/spf13/cobra/issues/745
func matchAll(checks ...cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		for _, check := range checks {
			if err := check(cmd, args); err != nil {
				return err
			}
		}
		return nil
	}
}

var projectUpdateCmd = &cobra.Command{
	Use:       "update [members] [quota]",
	Short:     "Updates project attributes with values from the project storage",
	Long:      ``,
	ValidArgs: []string{"members", "quota"},
	Args:      matchAll(cobra.OnlyValidArgs, cobra.MinimumNArgs(1)),
	RunE: func(cmd *cobra.Command, args []string) error {
		ipdb := loadPdb()
		conf := loadConfig()

		projects, err := ipdb.GetProjects(false)
		if err != nil {
			return err
		}

		log.Debugf("updating %s for %d projects", args, len(projects))

		// initialize filergateway client
		fgw, err := filergateway.NewClient(conf)
		if err != nil {
			return err
		}

		opts := make(map[string]struct{}, len(args))
		for _, o := range args {
			opts[o] = struct{}{}
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
						if _, x := opts["members"]; x {
							if err := v1.UpdateProjectMembers(pid, info.Members); err != nil {
								log.Errorf("[%s] cannot update members in pdb: %s", pid, err)
							}
						}
						if _, x := opts["quota"]; x {
							if err := v1.UpdateProjectStorageQuota(pid, info.Storage.QuotaGb, info.Storage.UsageMb>>10); err != nil {
								log.Errorf("[%s] cannot update storage usage in pdb: %s", pid, err)
							}
						}
					} else {
						log.Warnf("[%s] not pdb V1: skip update.", pid)
					}
				}
			}()
		}

		for _, project := range projects {
			cpids <- project.ID
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

		// perform pending actions sequencially as the NetApp API
		// doesn't seem to be able to handle it concurrently.
		for pid, action := range actions {
			if err := actionExec(pid, action); err != nil {
				log.Errorf("%s", err)
			}
		}

		// // perform pending actions with 4 concurrent workers,
		// // each works on a project.
		// var wg sync.WaitGroup
		// pids := make(chan string, execNthreads*2)
		// for w := 0; w < execNthreads; w++ {
		// 	wg.Add(1)
		// 	go func() {
		// 		defer wg.Done()
		// 		for pid := range pids {
		// 			if err := actionExec(pid, actions[pid]); err != nil {
		// 				log.Errorf("%s", err)
		// 			}
		// 		}
		// 	}()
		// }

		// for pid := range actions {
		// 	pids <- pid
		// }
		// close(pids)

		// // wait for all workers to finish
		// wg.Wait()

		return nil
	},
}

var projectAlertCmd = &cobra.Command{
	Use:   "alert",
	Short: "Utility for sending project-related alerts",
	Long:  ``,
}

var projectAlertOoqCmd = &cobra.Command{
	Use:   "ooq",
	Short: "Utility for project-storage out-of-quota alerts",
	Long:  ``,
}

// submcommand to show last alerts sent for projects (close to) running out of quota.
var projectAlertOoqInfo = &cobra.Command{
	Use:   "info",
	Short: "Shows information of the last alerts sent concerning project running out of quota",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {

		// check availability of the `alertDbPath`
		if _, err := os.Stat(alertDbPath); os.IsNotExist(err) {
			return fmt.Errorf("alert db not found: %s", alertDbPath)
		}

		// connect to internal database for last sent
		store := store.KVStore{
			Path: alertDbPath,
		}
		err := store.Connect()
		if err != nil {
			return err
		}
		defer store.Disconnect()

		// initialize kvstore with bucket "ooqLastAlerts"
		dbBucket := "ooqLastAlerts"
		err = store.Init([]string{dbBucket})
		if err != nil {
			return err
		}

		// gets all last alerts
		kvpairs, err := store.GetAll(dbBucket)
		if err != nil {
			return err
		}

		for _, kvpair := range kvpairs {
			pid := string(kvpair.Key)

			lastSent := pdb.OoqLastAlert{}
			err := json.Unmarshal(kvpair.Value, &lastSent)
			if err != nil {
				log.Errorf("[%s] cannot interpret ooq alert data: %s", pid, err)
				continue
			}

			fmt.Printf("%12s (%3d%%): %3d%% %s\n", pid, lastSent.UsagePercentLastCheck, lastSent.UsagePercent, lastSent.Timestamp)
		}

		return nil
	},
}

// submcommand to notify manager/contributor/owner when project is (close to) running out of quota.
var projectAlertOoqSend = &cobra.Command{
	Use:   "send",
	Short: "Sends alerts concerning project running out of quota",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {

		ipdb := loadPdb()
		conf := loadConfig()

		projects, err := ipdb.GetProjects(true)
		if err != nil {
			return err
		}

		log.Debugf("retriving storage usage for %d projects", len(projects))

		// initialize filergateway client
		fgw, err := filergateway.NewClient(conf)
		if err != nil {
			return err
		}

		// connect to internal database for last sent
		store := store.KVStore{
			Path: alertDbPath,
		}
		err = store.Connect()
		if err != nil {
			return err
		}
		defer store.Disconnect()

		// initialize kvstore with bucket "ooqLastAlerts"
		dbBucket := "ooqLastAlerts"
		err = store.Init([]string{dbBucket})
		if err != nil {
			return err
		}

		// perform pending actions with 4 concurrent workers,
		// each works on a project.
		var wg sync.WaitGroup
		cprjs := make(chan *pdb.Project, execNthreads*2)
		for w := 0; w < execNthreads; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				for prj := range cprjs {

					// perform test on a specific project with id specified via
					// `ooqAlertTestProjectID`.
					if ooqAlertTestProjectID != "" && prj.ID != ooqAlertTestProjectID {
						log.Infof("[%s] ignored ooq alert in test mode", prj.ID)
						continue
					}

					// get members from filer gateway
					info, err := fgw.GetProject(prj.ID)
					if err != nil {
						log.Errorf("[%s] cannot get project storage info: %s", prj.ID, err)
						continue
					}
					log.Debugf("[%s] project storage info: %+v", prj.ID, info)

					// get last alert information from the local db
					data, err := store.Get(dbBucket, []byte(prj.ID))
					if err != nil {
						log.Debugf("[%s] cannot get last ooq alert data: %s", prj.ID, err)
					}

					// default lastAlert with timestamp (0001-01-01 00:00:00 +0000 UTC), uratio 0.
					lastAlert := pdb.OoqLastAlert{}
					if data != nil {
						if err := json.Unmarshal(data, &lastAlert); err != nil {
							log.Debugf("[%s] cannot interpret last ooq alert data: %s", prj.ID, err)
						}
					}
					log.Debugf("[%s] last ooq alert: %+v", prj.ID, lastAlert)

					// check and send alert
					switch lastAlert, err = ooqAlert(ipdb, prj, info, lastAlert, conf.SMTP); err.(type) {
					case nil:
						log.Debugf("[%s] last ooq alert: %+v", prj.ID, lastAlert)
						// alert sent, update store db with new last alert information
						data, _ := json.Marshal(&lastAlert)
						store.Set(dbBucket, []byte(prj.ID), data)
					case *pdb.OpsIgnored:
						// alert ignored
						log.Debugf("[%s] %s", prj.ID, err)
						log.Debugf("[%s] last ooq alert: %+v", prj.ID, lastAlert)
						// alert ignored, still need to update the lastAlert to alert history db with
						// the current project UsagePercent.
						data, _ := json.Marshal(&lastAlert)
						store.Set(dbBucket, []byte(prj.ID), data)
					default:
						// something wrong
						log.Errorf("[%s] fail to send alert for project out-of-quota: +%v", prj.ID, err)
					}
				}
			}()
		}

		for _, project := range projects {
			cprjs <- project
		}
		close(cprjs)

		// wait for all workers to finish
		wg.Wait()

		return nil
	},
}

// ooqAlert checks whether alert concerning project storage out-of-quota
// is to be sent based on the project storage information `info`.
//
// If the alert email is sent, it returns the time at which the emails were sent.
//
// If the alert sending is ignored by design, the returned error is `OpsIgnored`.
func ooqAlert(ipdb pdb.PDB, prj *pdb.Project, info *pdb.DataProjectInfo, lastAlert pdb.OoqLastAlert, smtpConfig config.SMTPConfiguration) (pdb.OoqLastAlert, error) {

	uratio := 100 * info.Storage.UsageMb / (info.Storage.QuotaGb << 10)

	// check if the usage is above the alert threshold.
	duration := ooqAlertFrequency(uratio)
	if duration == 0 {
		msg := fmt.Sprintf("usage (%d%%) below the ooq threshold.", uratio)
		lastAlert.UsagePercentLastCheck = uratio
		return lastAlert, &pdb.OpsIgnored{Message: msg}
	}

	// check if current usage ratio is higher than the usage ratio at the time the last alert was sent.
	minUsageLastAert := lastAlert.UsagePercent
	if lastAlert.UsagePercentLastCheck < minUsageLastAert {
		minUsageLastAert = lastAlert.UsagePercentLastCheck
	}
	if uratio < minUsageLastAert {
		msg := fmt.Sprintf("usage (%d%%) below the usage (%d%%) at the last alert/check.", uratio, minUsageLastAert)
		lastAlert.UsagePercentLastCheck = uratio
		return lastAlert, &pdb.OpsIgnored{Message: msg}
	}

	// check if a new alert should be sent according to the alert frequency.
	now := time.Now()
	next := lastAlert.Timestamp.Add(duration)
	if now.Before(next) { // current time is in between
		msg := fmt.Sprintf("%s not reaching next alert %s.", now, next)
		lastAlert.UsagePercentLastCheck = uratio
		return lastAlert, &pdb.OpsIgnored{Message: msg}
	}

	// initializing mailer
	mailer := mailer.New(smtpConfig)

	// gather user ids of potential recipients
	recipients := make(map[string]struct{})
	recipients[prj.Owner] = struct{}{}

	for _, m := range info.Members {
		if m.Role == acl.Manager.String() || m.Role == acl.Contributor.String() {
			recipients[m.UserID] = struct{}{}
		}
	}

	// sending alerts to recipients
	nsent := 0
	for r := range recipients {
		u, err := ipdb.GetUser(r)
		if err != nil {
			log.Errorf("[%s] cannot get recipient info from project database: %s", info.ProjectID, r)
			continue
		}

		if ooqAlertSkipPI && u.Function == pdb.UserFunctionPrincipalInvestigator {
			log.Debugf("[%s] skip alert to PI: %s", info.ProjectID, u.ID)
			continue
		}

		log.Debugf("[%s] alert %s on usage ratio: %d", info.ProjectID, u.Email, uratio)

		if err := mailer.AlertProjectStorageOoq(*u, info.Storage, info.ProjectID, prj.Name); err != nil {
			log.Errorf("[%s] fail to sent ooq alert to %s: %s", info.ProjectID, u.Email, err)
		}

		nsent++
	}

	// return `pdb.OpsIgnored` if no alert was (successfully) sent.
	if nsent == 0 {
		lastAlert.UsagePercentLastCheck = uratio
		return lastAlert, &pdb.OpsIgnored{Message: "no alert was sent"}
	}

	return pdb.OoqLastAlert{
		Timestamp:             now,
		UsagePercent:          uratio,
		UsagePercentLastCheck: uratio,
	}, nil
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

	// check if it is about an existing project
	fgw, err := filergateway.NewClient(conf)
	if err != nil {
		return err
	}

	newProject := true
	if _, err = fgw.GetProject(pid); err == nil {
		newProject = false
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
		// create/update project storage via filer-gateway

		if newProject {
			// synchronously create new project
			if _, err := fgw.SyncCreateProject(pid, act, time.Second); err != nil {
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

		} else {
			// synchronously update existing project
			if _, err := fgw.SyncUpdateProject(pid, act, time.Second); err != nil {
				return fmt.Errorf("[%s] failure updating project: %s", pid, err)
			}
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

	// For PDBv1, get ACL from the filer gateway and update database accordingly.
	if v1, ok := ipdb.(pdb.V1); ok {

		pdata, err := fgw.GetProject(pid)

		if err != nil {
			return fmt.Errorf("[%s] fail getting acl: %s", pid, err)
		}

		// construct data structure for updating PDB v1 database.
		members := pdata.Members

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

		// get project name needed for the notification email
		p, err := ipdb.GetProject(pid)
		if err != nil {
			return fmt.Errorf("[%s] fail getting project detail for notification: %s", pid, err)
		}

		mailer := mailer.New(conf.SMTP)
		for _, m := range managers {

			log.Debugf("[%s] sending notification to manager %s", pid, m)

			u, err := ipdb.GetUser(m)

			if err != nil {
				log.Errorf("[%s] fail getting user profile of manager %s: %s", pid, m, err)
				continue
			}

			if err := mailer.NotifyProjectProvisioned(*u, pid, p.Name); err != nil {
				log.Errorf("[%s] fail notifying manager %s: %s", pid, m, err)
			}
		}
	}

	return nil
}
