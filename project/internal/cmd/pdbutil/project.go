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

	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
	log "github.com/dccn-tg/tg-toolset-golang/pkg/logger"
	"github.com/dccn-tg/tg-toolset-golang/pkg/mailer"
	"github.com/dccn-tg/tg-toolset-golang/pkg/store"
	"github.com/dccn-tg/tg-toolset-golang/project/pkg/acl"
	"github.com/dccn-tg/tg-toolset-golang/project/pkg/filergateway"
	"github.com/dccn-tg/tg-toolset-golang/project/pkg/pdb"
	"github.com/spf13/cobra"
)

const dateLayout string = "2006-01-02"

var (
	now               time.Time = time.Now()
	execNthreads      int       = 4
	activeProjectOnly bool      = false
	useNetappCLI      bool      = false

	storSystem   string            = "netapp"
	projectRoots map[string]string = map[string]string{
		"netapp":  "/project",
		"freenas": "/project_freenas",
		"cephfs":  "/project_cephfs",
	}
	alertDbPath          string = "alert.db"
	alertTestProjectID   string
	alertSkipPI          bool     = false
	alertDryrun          bool     = false
	alertMode            string   = "p4w"
	alertSender          string   = "DCCN TG Helpdesk"
	alertSenderEmail     string   = "helpdesk@donders.ru.nl"
	alertCarbonCopy      string   = "rene.debruin@donders.ru.nl"
	alertSkipContributor []string = []string{}

	skipContributorPmap map[string]bool = make(map[string]bool)

	// ootAlertDate calculates the project expiry alerting dates for:
	// - "p4w": 28-day in advance
	// - "p23": 14-day in advance
	// - "now": on the expiration date
	// - "g1m": 30-day grace time
	// - "g2m": 60-day gracen time
	ootAlertDate map[string]string = map[string]string{
		"p4w": now.AddDate(0, 0, 28).Format(dateLayout),
		"p2w": now.AddDate(0, 0, 14).Format(dateLayout),
		"p1w": now.AddDate(0, 0, 7).Format(dateLayout),
		"now": now.Format(dateLayout),
		"g2m": now.AddDate(0, -2, 0).Format(dateLayout),
	}
)

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

	// get supported storage systems from the `ootAlertDate`.
	supportedAlertModes := make([]string, len(ootAlertDate))
	i = 0
	for mode := range ootAlertDate {
		supportedAlertModes[i] = mode
		i++
	}

	projectActionExecCmd.Flags().IntVarP(&execNthreads, "nthreads", "n", execNthreads,
		"`number` of concurrent worker threads.")

	projectActionExecCmd.Flags().BoolVarP(&useNetappCLI, "netapp-cli", "", useNetappCLI,
		"use NetApp ONTAP CLI to apply changes on the NetApp filer. Only applicable for the netapp storage system.")

	projectActionExecCmd.Flags().StringVarP(&storSystem, "sys", "s", storSystem,
		fmt.Sprintf("storage `system`.  Supported systems: %s", strings.Join(supportedStorSystems, ",")))

	projectCreateCmd.Flags().StringVarP(&storSystem, "sys", "s", storSystem,
		fmt.Sprintf("storage `system`.  Supported systems: %s", strings.Join(supportedStorSystems, ",")))

	projectUpdateCmd.Flags().BoolVarP(&activeProjectOnly, "active-only", "a", activeProjectOnly,
		"update only the active projects")

	projectAlertCmd.PersistentFlags().StringVarP(&alertDbPath, "dbpath", "", alertDbPath,
		"`path` of the internal alert history database")

	projectAlertCmd.PersistentFlags().StringVarP(&alertTestProjectID, "test", "t", alertTestProjectID,
		"`id` of project for testing ooq alert")

	projectAlertCmd.PersistentFlags().StringVarP(&alertSender, "sender", "", alertSender,
		"`name` of the alert sender")

	projectAlertCmd.PersistentFlags().StringVarP(&alertSenderEmail, "from", "f", alertSenderEmail,
		"`email` of the alert sender")

	projectAlertCmd.PersistentFlags().BoolVarP(&alertSkipPI, "skip-pi", "", alertSkipPI,
		"set to skip sending alert to PIs")

	projectAlertCmd.PersistentFlags().StringSliceVarP(&alertSkipContributor, "skip-contributor", "", alertSkipContributor,
		"specify a list of comma-separated projects of which the contributors are skipped for alert")

	projectAlertCmd.PersistentFlags().StringVarP(&alertCarbonCopy, "cc", "", alertCarbonCopy,
		"alert carbon copy `email`")

	projectAlertCmd.PersistentFlags().BoolVarP(&alertDryrun, "dryrun", "", alertDryrun,
		"print out alerts and recipients without really sent them")

	projectAlertOotCmd.PersistentFlags().StringVarP(&alertMode, "mode", "m", alertMode,
		fmt.Sprintf("alert `mode`. Supported modes: %s", strings.Join(supportedAlertModes, ",")))

	projectActionCmd.AddCommand(projectActionListCmd, projectActionExecCmd)

	projectAlertOoqCmd.AddCommand(projectAlertOoqInfo, projectAlertOoqSend)

	projectAlertOotCmd.AddCommand(projectAlertOotInfo, projectAlertOotSend)

	projectAlertCmd.AddCommand(projectAlertOoqCmd, projectAlertOotCmd)

	projectCmd.AddCommand(projectActionCmd, projectCreateCmd, projectUpdateCmd, projectAlertCmd)

	rootCmd.AddCommand(projectCmd)
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Utility for managing project storage",
	Long:  ``,
}

var projectCreateCmd = &cobra.Command{
	Use:   "create [projectID] [quotaGB]",
	Short: "Creates or updates project storage with given quota",
	Long:  ``,
	Args:  cobra.ExactArgs(2),
	PreRun: func(cmd *cobra.Command, args []string) {
		if _, ok := projectRoots[storSystem]; !ok {
			log.Fatalf("unsupported storage system: %s", storSystem)
		}
	},
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
//
//	see https://github.com/spf13/cobra/issues/745
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
	PreRun: func(cmd *cobra.Command, args []string) {
		if _, ok := projectRoots[storSystem]; !ok {
			log.Fatalf("unsupported storage system: %s", storSystem)
		}
		rootCmd.PreRun(cmd, args)
	},
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

		return nil
	},
}

var projectAlertCmd = &cobra.Command{
	Use:   "alert",
	Short: "Utility for sending project-related alerts",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {

		rootCmd.PersistentPreRun(cmd, args)

		// convert input slice `alertSkipContributor` into internal map
		for _, p := range alertSkipContributor {
			skipContributorPmap[p] = true
		}

		log.Debugf("projects to skip contributor: %+v\n", skipContributorPmap)
	},
}

var projectAlertOotCmd = &cobra.Command{
	Use:   "oot",
	Short: "Utility for expiring or overdue (i.e. out-of-time) project alerts",
	Long:  ``,
}

// submcommand to show last alerts sent for projects (close to) running out of time.
var projectAlertOotInfo = &cobra.Command{
	Use:   "info",
	Short: "Shows information of the last alerts sent concerning expiring or overdue projects",
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

		// initialize kvstore with bucket "ootLastAlerts"
		dbBucket := "ootLastAlerts"
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

			lastSent := pdb.OotLastAlert{}
			err := json.Unmarshal(kvpair.Value, &lastSent)
			if err != nil {
				log.Errorf("[%s] cannot interpret oot alert data: %s", pid, err)
				continue
			}

			fmt.Printf("%12s: %s\n", pid, lastSent.Timestamp)
		}

		return nil
	},
}

// submcommand to notify manager/contributor/owner when project is (close to) running out of time.
var projectAlertOotSend = &cobra.Command{
	Use:   "send",
	Short: "Sends alerts concerning expiring or overdue projects",
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

		// initialize kvstore with bucket "ootLastAlerts"
		dbBucket := "ootLastAlerts"
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
					// `ootAlertTestProjectID`.
					if alertTestProjectID != "" && prj.ID != alertTestProjectID {
						log.Infof("[%s] ignored oot alert in test mode", prj.ID)
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
						log.Debugf("[%s] cannot get last oot alert data: %s", prj.ID, err)
					}

					// default lastAlert with timestamp (0001-01-01 00:00:00 +0000 UTC), uratio 0.
					lastAlert := pdb.OotLastAlert{}
					if data != nil {
						if err := json.Unmarshal(data, &lastAlert); err != nil {
							log.Debugf("[%s] cannot interpret last oot alert data: %s", prj.ID, err)
						}
					}
					log.Debugf("[%s] last oot alert: %+v", prj.ID, lastAlert)

					// check and send alert
					lastAlert, err = ootAlert(ipdb, prj, info, lastAlert, conf.SMTP)

					// do nothing to the db for dryrun
					if alertDryrun {
						continue
					}

					switch err.(type) {
					case nil:
						log.Debugf("[%s] last oot alert: %+v", prj.ID, lastAlert)
						// alert sent, update store db with new last alert information
						data, _ := json.Marshal(&lastAlert)
						store.Set(dbBucket, []byte(prj.ID), data)
					case *pdb.OpsIgnored:
						// alert ignored
						log.Debugf("[%s] %s", prj.ID, err)
						log.Debugf("[%s] last oot alert: %+v", prj.ID, lastAlert)
					default:
						// something wrong
						log.Errorf("[%s] fail to send alert for project out-of-time: +%v", prj.ID, err)
					}
				}
			}()
		}

		// select projects matching the alerting mode criteria
		for _, project := range projects {
			if project.End.Format(dateLayout) == ootAlertDate[alertMode] {
				cprjs <- project
			} else {
				log.Debugf("[%s] skipped as the end time (%s) doesn't match alerting criteria for mode %s", project.ID, project.End, alertMode)
			}
		}
		close(cprjs)

		wg.Wait()

		return nil
	},
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
					if alertTestProjectID != "" && prj.ID != alertTestProjectID {
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
					lastAlert, err = ooqAlert(ipdb, prj, info, lastAlert, conf.SMTP)

					// do nothing to the db for dryrun
					if alertDryrun {
						continue
					}

					switch err.(type) {
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

		wg.Wait()

		return nil
	},
}

// ootAlert sents out alert email concerning project going to expire at maximum frequency of
// once per week.
//
// If the alert email is sent, it returns the time at which the email were sent.
//
// If the alert sending is ignored, the returned error is `OpsIgnored`.
func ootAlert(ipdb pdb.PDB, prj *pdb.Project, info *pdb.DataProjectInfo, lastAlert pdb.OotLastAlert, smtpConfig config.SMTPConfiguration) (pdb.OotLastAlert, error) {

	now := time.Now()
	next := lastAlert.Timestamp.AddDate(0, 0, 3)

	// prevent sending alert if it has been sent once in the last 3 days
	// NOTE: this is a redundent protection given that the ootAlert implements `mode` to
	//       send out alert on an exact dates based on the project end time (i.e. 28/14/7/0 days in advance)
	if now.After(next) {
		// send the email
		// initializing mailer
		m := mailer.New(smtpConfig)

		// gather user information of project owner
		recipients := make(map[string]*pdb.User)
		u, err := ipdb.GetUser(prj.Owner)
		switch {
		case err != nil:
			log.Errorf("[%s] cannot get recipient info from project database: %s", info.ProjectID, prj.Owner)
		case u.Status != pdb.UserStatusCheckedIn && u.Status != pdb.UserStatusCheckedOutExtended:
			log.Debugf("[%s] skip alert %s due to user state %s", info.ProjectID, u.ID, u.Status)
		default:
			recipients[prj.Owner] = u
		}

		// gather user information of project members
		for _, m := range info.Members {

			if m.Role != acl.Manager.String() && m.Role != acl.Contributor.String() {
				log.Debugf("[%s] skip alert %s due to user role %s", info.ProjectID, m.UserID, m.Role)
				continue
			}

			_, skipc := skipContributorPmap[info.ProjectID]
			if m.Role == acl.Contributor.String() && skipc {
				log.Debugf("[%s] skip alert to contributor %s", info.ProjectID, m.UserID)
				continue
			}

			u, err := ipdb.GetUser(m.UserID)
			switch {
			case err != nil:
				log.Errorf("[%s] cannot get recipient info from project database: %s", info.ProjectID, m.UserID)
			case u.Status != pdb.UserStatusCheckedIn && u.Status != pdb.UserStatusCheckedOutExtended:
				log.Debugf("[%s] skip alert %s due to user state %s", info.ProjectID, u.ID, u.Status)
			default:
				recipients[m.UserID] = u
			}
		}

		// sending alerts to recipients
		nsent := 0
		data := mailer.ProjectAlertTemplateData{
			ProjectID:      info.ProjectID,
			ProjectTitle:   prj.Name,
			ProjectEndDate: prj.End.Format(dateLayout),
			SenderName:     alertSender,
		}

		for _, u := range recipients {

			// only apply `alertSkipPI` option if
			// - the PI is not the project owner, and
			// - there are more than 1 alert recipient
			if alertSkipPI && u.Function == pdb.UserFunctionPrincipalInvestigator && u.ID != prj.Owner && len(recipients) > 1 {
				log.Debugf("[%s] skip alert PI: %s", info.ProjectID, u.ID)
				continue
			}

			data.RecipientName = u.DisplayName()

			// compose alert subject and body
			var subject, body string
			switch alertMode {
			case "p4w":
				data.ExpiringInDays = 28
				subject, body, err = mailer.ComposeProjectExpiringAlert(data)
			case "p2w":
				data.ExpiringInDays = 14
				subject, body, err = mailer.ComposeProjectExpiringAlert(data)
			case "p1w":
				data.ExpiringInDays = 7
				subject, body, err = mailer.ComposeProjectExpiringAlert(data)
			case "now":
				data.ExpiringInDays = 0
				subject, body, err = mailer.ComposeProjectExpiringAlert(data)
			case "g2m":
				data.ExpiringInMonths = -2
				subject, body, err = mailer.ComposeProjectEndOfGracePeriodAlert(data)
			default:
				// ignore operation if alertMode is not a defined mode
				msg := fmt.Sprintf("ignore unknown alert mode %s", alertMode)
				return lastAlert, &pdb.OpsIgnored{Message: msg}
			}

			if err != nil {
				log.Debugf("[%s] skip alert %s due to failure generating alert: %s", info.ProjectID, u.ID, err)
				continue
			}

			if alertDryrun {
				log.Infof("[%s] alert %s", info.ProjectID, u.Email)
			} else if err := m.SendMail(alertSenderEmail, subject, body, []string{u.Email}, alertCarbonCopy); err != nil {
				log.Errorf("[%s] fail to sent oot alert to %s: %s", info.ProjectID, u.Email, err)
			}

			nsent++
		}

		return pdb.OotLastAlert{
			Timestamp: now,
		}, nil
	}
	msg := fmt.Sprintf("last alert has been sent on %s", lastAlert.Timestamp.Format(dateLayout))
	return lastAlert, &pdb.OpsIgnored{Message: msg}
}

// ooqAlert checks whether alert concerning project storage out-of-quota
// is to be sent based on the project storage information `info`.
//
// If the alert email is sent, it returns the time at which the emails were sent.
//
// If the alert sending is ignored by design, the returned error is `OpsIgnored`.
func ooqAlert(ipdb pdb.PDB, prj *pdb.Project, info *pdb.DataProjectInfo, lastAlert pdb.OoqLastAlert, smtpConfig config.SMTPConfiguration) (pdb.OoqLastAlert, error) {

	var uratio int

	switch {
	case info.Storage.QuotaGb == 0 && info.Storage.UsageMb > 0:
		uratio = 100
	case info.Storage.QuotaGb == 0 && info.Storage.UsageMb == 0:
		uratio = 0
	default:
		uratio = 100 * info.Storage.UsageMb / (info.Storage.QuotaGb << 10)
	}

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
	m := mailer.New(smtpConfig)

	// gather user information of project owner
	recipients := make(map[string]*pdb.User)

	u, err := ipdb.GetUser(prj.Owner)
	switch {
	case err != nil:
		log.Errorf("[%s] cannot get recipient info from project database: %s", info.ProjectID, prj.Owner)
	case u.Status != pdb.UserStatusCheckedIn && u.Status != pdb.UserStatusCheckedOutExtended:
		log.Debugf("[%s] skip alert %s due to user state %s", info.ProjectID, u.ID, u.Status)
	default:
		recipients[prj.Owner] = u
	}

	// gather user information of project members
	for _, m := range info.Members {

		if m.Role != acl.Manager.String() && m.Role != acl.Contributor.String() {
			log.Debugf("[%s] skip alert %s due to user role %s", info.ProjectID, m.UserID, m.Role)
			continue
		}

		_, skipc := skipContributorPmap[info.ProjectID]
		if m.Role == acl.Contributor.String() && skipc {
			log.Debugf("[%s] skip alert to contributor %s", info.ProjectID, m.UserID)
			continue
		}

		u, err := ipdb.GetUser(m.UserID)
		switch {
		case err != nil:
			log.Errorf("[%s] cannot get recipient info from project database: %s", info.ProjectID, m.UserID)
		case u.Status != pdb.UserStatusCheckedIn && u.Status != pdb.UserStatusCheckedOutExtended:
			log.Debugf("[%s] skip alert %s due to user state %s", info.ProjectID, u.ID, u.Status)
		default:
			recipients[m.UserID] = u
		}
	}

	// sending alerts to recipients
	nsent := 0
	data := mailer.ProjectAlertTemplateData{
		ProjectID:       info.ProjectID,
		ProjectTitle:    prj.Name,
		QuotaUsageRatio: uratio,
		SenderName:      alertSender,
	}
	for _, u := range recipients {

		// shall we ignore `skip-pi` option if PI is the only recipient??
		if alertSkipPI && u.Function == pdb.UserFunctionPrincipalInvestigator {
			log.Debugf("[%s] skip alert PI: %s", info.ProjectID, u.ID)
			continue
		}

		data.RecipientName = u.DisplayName()

		subject, body, err := mailer.ComposeProjectOutOfQuotaAlert(data)

		if err != nil {
			log.Debugf("[%s] skip alert %s due to failure generating alert: %s", info.ProjectID, u.ID, err)
			continue
		}

		if alertDryrun {
			log.Infof("[%s] alert %s on usage ratio: %d", info.ProjectID, u.Email, uratio)
		} else if err := m.SendMail(alertSenderEmail, subject, body, []string{u.Email}); err != nil {
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

		// wait for 5 seconds to give the filer-gateway time to refresh the cache
		// upon project update.
		time.Sleep(5 * time.Second)

		pdata, err := fgw.GetProject(pid)

		if err != nil {
			return fmt.Errorf("[%s] fail getting acl: %s", pid, err)
		}

		// construct data structure for updating PDB v1 database.
		members := pdata.Members

		log.Debugf("[%s] retrieved acl from filergateway: %+v", pid, members)

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

		data := mailer.ProjectAlertTemplateData{
			ProjectID:    pid,
			ProjectTitle: p.Name,
			SenderName:   alertSender,
		}

		m := mailer.New(conf.SMTP)
		for _, manager := range managers {

			log.Debugf("[%s] sending notification to manager %s", pid, manager)

			u, err := ipdb.GetUser(manager)

			data.RecipientName = u.DisplayName()

			if err != nil {
				log.Errorf("[%s] fail getting user profile of manager %s: %s", pid, manager, err)
				continue
			}

			subject, body, err := mailer.ComposeProjectProvisionedAlert(data)

			if err != nil {
				log.Debugf("[%s] skip notify %s due to failure generating alert: %s", pid, u.ID, err)
				continue
			}

			if err := m.SendMail(alertSenderEmail, subject, body, []string{u.Email}); err != nil {
				log.Errorf("[%s] fail notifying manager %s: %s", pid, m, err)
			}
		}
	}

	return nil
}
