// Package pdb provides functions for interacting with the database of the project database.
package pdb

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
	log "github.com/dccn-tg/tg-toolset-golang/pkg/logger"
	"github.com/go-sql-driver/mysql"
)

// V1 implements interfaces of the legacy project database implemented with MySQL database.
type V1 struct {
	config config.DBConfiguration
}

func (v1 V1) GetUsers(activeOnly bool) ([]*User, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetProjectPendingActions performs queries to get project pending roles and project storage
// resource, and combines the results into a data structure that can be directly used for
// sending project update request to the filer-gateway API:
// https://github.com/dccn-tg/filer-gateway
func (v1 V1) GetProjectPendingActions() (map[string]*DataProjectUpdate, error) {

	actions := make(map[string]*DataProjectUpdate)

	db, err := newClientMySQL(v1.config)
	if err != nil {
		return actions, err
	}

	// make sure the db client is closed before exiting the function.
	defer db.Close()

	query := `
	SELECT
		a.user_id,a.project_id,a.role,a.created,a.action,b.calculatedProjectSpace
	FROM
		projectmembers as a,
		projects as b
	WHERE
		a.activated='no' AND 
		b.calculatedProjectSpace > 0 AND
		a.project_id=b.id
	ORDER BY
		a.created
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// internal data structure for raw action data
	type rawAction struct {
		pid        string
		uid        string
		role       string
		action     string
		quota      int
		createTime time.Time
	}

	// internal map holding valid rawActions
	rawActions := make(map[string]rawAction)

	for rows.Next() {
		var (
			uid    string
			pid    string
			role   string
			action string
			ctime  time.Time
			quota  int
		)

		if err := rows.Scan(&uid, &pid, &role, &ctime, &action, &quota); err != nil {
			return nil, err
		}

		// there is already a pending role action on pid+uid, check which one we should consider
		if a, ok := rawActions[pid+uid]; ok {
			// compare the existing action 'a' with the current one,
			// and pick up the one created later.
			if ctime.After(a.createTime) {
				rawActions[pid+uid] = rawAction{
					pid:        pid,
					uid:        uid,
					role:       role,
					action:     action,
					quota:      quota,
					createTime: ctime,
				}
			}
			continue
		}

		// this is new role action on pid+uid, add the action to the internal roleActionMap
		rawActions[pid+uid] = rawAction{
			pid:        pid,
			uid:        uid,
			role:       role,
			action:     action,
			quota:      quota,
			createTime: ctime,
		}

		log.Debugf("%s user %s to role %s in project %s", action, uid, role, pid)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// convert rawActions map into actions
	// NOTE: we are missing system parameter so that all actions are assumed to be
	//       executed on the `netapp`, the default filer system.
	for _, a := range rawActions {
		if _, ok := actions[a.pid]; !ok {
			actions[a.pid] = &DataProjectUpdate{
				Members: []Member{},
				Storage: Storage{
					QuotaGb: a.quota,
					System:  "netapp",
				},
			}
		}

		// set role to "none" if the action concerns role deletion.
		if a.action == "delete" {
			a.role = "none"
		}

		actions[a.pid].Members = append(actions[a.pid].Members, Member{
			UserID:    a.uid,
			Role:      a.role,
			Timestamp: a.createTime,
		})
	}

	return actions, nil
}

// DelProjectPendingActions performs deletion on the pending-role actions from the project
// database.
func (v1 V1) DelProjectPendingActions(actions map[string]*DataProjectUpdate) error {
	db, err := newClientMySQL(v1.config)
	if err != nil {
		return err
	}

	// make sure the db client is closed before exiting the function.
	defer db.Close()

	type rawAction struct {
		pid        string
		uid        string
		createTime time.Time
	}

	rawActions := make([]rawAction, 0)

	for pid, act := range actions {
		for _, m := range act.Members {
			rawActions = append(rawActions, rawAction{
				pid:        pid,
				uid:        m.UserID,
				createTime: m.Timestamp,
			})
		}
	}

	// prepare sql to set actions concerning pid/uid created before the creation timestamp of
	// the perfomed actions.
	query := `
	UPDATE
		projectmembers
	SET
		activated=?, updated=?
	WHERE
		project_id=? AND user_id=? AND created<=?
	`

	stmt, err := db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, a := range rawActions {
		if _, err := stmt.Exec("yes", time.Now(), a.pid, a.uid, a.createTime); err != nil {
			return err
		}
	}

	return nil
}

// GetProjects retrieves list of project identifiers from the project database.
func (v1 V1) GetProjects(activeOnly bool) ([]*Project, error) {

	db, err := newClientMySQL(v1.config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// prepare db query
	query := `
	SELECT
		id, projectName, owner_id, calculatedProjectSpace
	FROM
		projects
	`

	if activeOnly {
		query += `
		WHERE
			calculatedProjectSpace > 0
		`
	}

	projects := make([]*Project, 0)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var pid string
		var pname string
		var oid string
		var cspace int
		err := rows.Scan(&pid, &pname, &oid, &cspace)
		if err != nil {
			return nil, err
		}

		projects = append(projects, &Project{
			ID:     pid,
			Name:   pname,
			Owner:  oid,
			Status: parseProjectStatusByCalculatedSpace(cspace),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return projects, nil
}

// GetProject retrieves attributes of a project.
func (v1 V1) GetProject(projectID string) (*Project, error) {

	db, err := newClientMySQL(v1.config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// prepare db query
	query := `
	SELECT
		id, projectName, owner_id, calculatedProjectSpace
	FROM
		projects
	WHERE
		id=?
	`

	stmt, err := db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var pid string
	var pname string
	var oid string
	var cspace int

	err = stmt.QueryRow(projectID).Scan(&pid, &pname, &oid, &cspace)
	if err != nil {
		return nil, err
	}

	return &Project{
		ID:     pid,
		Name:   pname,
		Owner:  oid,
		Status: parseProjectStatusByCalculatedSpace(cspace),
	}, nil
}

// parseProjectStatusByCalculatedSpace interprets the project status by
// the calculated space.
func parseProjectStatusByCalculatedSpace(space int) ProjectStatus {
	if space > 0 {
		return ProjectStatusActive
	}
	return ProjectStatusInactive
}

// UpdateProjectMembers updates the project database with the given project roles.
func (v1 V1) UpdateProjectMembers(project string, members []Member) error {

	db, err := newClientMySQL(v1.config)
	if err != nil {
		return err
	}
	defer db.Close()

	return updateProjectRoles(db, project, members)
}

// UpdateProjectStorageQuota updates the project database with the current project storage usage.
func (v1 V1) UpdateProjectStorageQuota(project string, quotaGB, usageGB int) error {

	db, err := newClientMySQL(v1.config)
	if err != nil {
		return err
	}
	defer db.Close()

	return updateQuota(db, project, quotaGB, usageGB)
}

// GetUser gets the user identified by the given uid in the project database.
// It returns the pointer to the user data represented in the User data structure.
func (v1 V1) GetUser(uid string) (*User, error) {

	db, err := newClientMySQL(v1.config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return selectUser(db, "id = ?", uid)
}

// GetUserByEmail gets the user identified by the given email address.
func (v1 V1) GetUserByEmail(email string) (*User, error) {
	db, err := newClientMySQL(v1.config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// MariaDB query in case insensitive way; no need for case conversion.
	return selectUser(db, "email = ?", email)
}

// GetLabBookingsForWorklist retrieves TENTATIVE and CONFIRMED calendar bookings concerning the given `Lab` on a given `date` string.
// The `date` string is in the format of `2020-04-22`.
func (v1 V1) GetLabBookingsForWorklist(lab Lab, date string) ([]*LabBooking, error) {
	bookings := make([]*LabBooking, 0)

	db, err := newClientMySQL(v1.config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
	SELECT
		a.id,a.project_id,a.subj_ses,a.start_time,a.stop_time,a.status,a.user_id,b.projectName,c.description
	FROM
		calendar_items_new AS a,
		projects AS b,
		calendars AS c
	WHERE
		a.status IN ('CONFIRMED','TENTATIVE') AND
		a.subj_ses NOT IN ('Cancellation','0') AND
		a.start_date = ? AND
		a.project_id = b.id AND
		a.calendar_id = c.id
	ORDER BY
		a.start_time
	`

	rows, err := db.Query(query, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// regular expression for matching lab description
	labPat, err := lab.GetDescriptionRegex()
	if err != nil {
		return nil, err
	}

	// regular expression for spliting subject and session identifiers
	subjsesSpliter := regexp.MustCompile(`\s*(-)\s*`)

	// loop over results of the query
	for rows.Next() {
		var (
			id      string
			pid     string
			subjSes string
			stime   []uint8
			etime   []uint8
			status  string
			uid     string
			pname   string
			labdesc string
		)

		err := rows.Scan(&id, &pid, &subjSes, &stime, &etime, &status, &uid, &pname, &labdesc)
		if err != nil {
			return nil, err
		}

		log.Debugf("%s %s %s %s", id, pid, subjSes, labdesc)

		if m := labPat.FindStringSubmatch(strings.ToUpper(labdesc)); len(m) >= 2 {
			var (
				subj string
				sess string
			)
			if dss := subjsesSpliter.Split(subjSes, -1); len(dss) < 2 {
				subj = dss[0]
				sess = "1"
			} else {
				subj = dss[0]
				sess = dss[1]
			}

			ststr := fmt.Sprintf("%sT%s", date, stime)
			st, err := time.Parse(time.RFC3339[:19], ststr)
			if err != nil {
				log.Errorf("cannot parse start time: %s", ststr)
				continue
			}

			etstr := fmt.Sprintf("%sT%s", date, etime)
			et, err := time.Parse(time.RFC3339[:19], etstr)
			if err != nil {
				log.Errorf("cannot parse end time: %s", etstr)
				continue
			}

			pdbUser, err := selectUser(db, "id = ?", uid)
			if err != nil {
				log.Errorf("cannot find user in PDB: %s", uid)
				continue
			}

			bookings = append(bookings, &LabBooking{
				Project:      pid,
				Subject:      subj,
				Session:      sess,
				Lab:          m[1],
				ProjectTitle: pname,
				Operator:     *pdbUser,
				StartTime:    st,
				EndTime:      et,
				Status:       status,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return bookings, nil
}

func (v1 V1) GetLabBookingsForReport(lab Lab, from, to string) ([]*LabBooking, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetExperimentersForSharedAnatomicalMR retrieves a list of experimenters that are
// allowed to access to the shared anatomical MR data at this moment.
//
// Those are experiments of projects that are conducting data acquisition using the
// EEG and MEG modalities.
func (v1 V1) GetExperimentersForSharedAnatomicalMR() ([]*User, error) {
	db, err := newClientMySQL(v1.config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
	SELECT
    	DISTINCT id,firstName,middleName,lastName,email,function,status
	FROM
    	users
	WHERE
		id IN (
			SELECT 
				experimenter_id
			FROM 
				experiments
			WHERE 
				imaging_method_id IN ('eeg','meg151','meg275') AND
				date_sub(startingDate, interval 3 month) < now() AND
				date_add(endingDate, interval 3 month) > now()
		) AND
		status IN ('checked in','checked out extended')
	GROUP BY id;
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	experimenters := make([]*User, 0)

	for rows.Next() {

		var (
			id         string
			firstname  string
			middlename string
			lastname   string
			email      string
			function   string
			status     string
		)

		if err := rows.Scan(&id, &firstname, &middlename, &lastname, &email, &function, &status); err != nil {
			return nil, err
		}

		experimenters = append(experimenters, &User{
			ID:         id,
			Firstname:  firstname,
			Middlename: middlename,
			Lastname:   lastname,
			Email:      email,
			Function:   parseUserFunction(function),
			Status:     parseUserStatus(status),
		})
	}

	return experimenters, nil
}

// newClientMySQL establishes the MySQL client connection with the configuration.
func newClientMySQL(config config.DBConfiguration) (*sql.DB, error) {
	mycfg := mysql.Config{
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", config.HostSQL, config.PortSQL),
		DBName:               config.DatabaseSQL,
		User:                 config.UserSQL,
		Passwd:               config.PassSQL,
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	return sql.Open("mysql", mycfg.FormatDSN())
}

// updateProjectRoles updates the registry of the data-access roles of the given project
// in the project database, according to the roles provided as a acl.RoleMap.
func updateProjectRoles(db *sql.DB, project string, members []Member) error {

	if err := db.Ping(); err != nil {
		return fmt.Errorf("PDB not connected")
	}

	// variables for transaction statements
	var (
		delStmt *sql.Stmt
		setStmt *sql.Stmt
	)

	// start db transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// defer function to ensure statements and transactions are closed properly
	defer func() {
		if delStmt != nil {
			delStmt.Close()
		}
		if setStmt != nil {
			setStmt.Close()
		}
		if err != nil {
			tx.Rollback()
		}
		err = tx.Commit()
	}()

	// delete all acls from the project
	delStmt, err = tx.Prepare("DELETE FROM acls WHERE project=?")
	if err != nil {
		return err
	}
	_, err = delStmt.Exec(project)
	if err != nil {
		return err
	}

	// insert new roles into the project
	setStmt, err = tx.Prepare("INSERT INTO acls (project, projectRole, user) VALUES (?,?,?)")
	if err != nil {
		return err
	}
	for _, m := range members {
		// check if the user in question is available in the project database.
		if _, err := selectUser(db, "id = ?", m.UserID); err != nil {
			// ignore user cannot be found in the project database.
			log.Warnf("cannot found users in pdb: %s, reason: %+v", m.UserID, err)
			continue
		}
		log.Debugf("Updating project %s, %s: %s", project, m.Role, m.UserID)
		_, err := setStmt.Exec(project, m.Role, m.UserID)
		if err != nil {
			return err
		}
	}

	return err
}

// updateQuota updates current quota usage of the given project.
func updateQuota(db *sql.DB, project string, quotaGB, usageGB int) error {

	if err := db.Ping(); err != nil {
		return fmt.Errorf("PDB not connected")
	}

	query := `
		UPDATE
			projects
		SET
			totalProjectSpace=?, usedProjectSpace=?
		WHERE
			id=?
	`

	stmt, err := db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	log.Debugf("Updating quota of project %s, total: %d, usage: %d", project, quotaGB, usageGB)
	_, err = stmt.Exec(quotaGB, usageGB, project)

	return err
}

// selectUser gets the user identified by the given uid in the project database.
//
// The input `clauseCond` and `clauseValue` should
// It returns the pointer to the user data represented in the User data structure.
func selectUser(db *sql.DB, clauseCond string, clauseValues ...interface{}) (*User, error) {

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("PDB not connected: %+v", err)
	}

	query := fmt.Sprintf(`
	SELECT
		id,firstName,middleName,lastName,email,function,status
	FROM
		users
	WHERE %s
	`, clauseCond)

	var (
		id         string
		firstname  string
		middlename string
		lastname   string
		email      string
		function   string
		status     string
	)

	if err := db.QueryRow(query, clauseValues...).Scan(&id, &firstname, &middlename, &lastname, &email, &function, &status); err != nil {
		return nil, err
	}

	return &User{
		ID:         id,
		Firstname:  firstname,
		Middlename: middlename,
		Lastname:   lastname,
		Email:      email,
		Function:   parseUserFunction(function),
		Status:     parseUserStatus(status),
	}, nil
}

// parseUserFunction interprets PDBv1 user function string into corresponding `UserFunction`.
// TODO: implement fine-grained user functions.
func parseUserFunction(f string) UserFunction {
	switch f {
	case "pi", "Principal Investigator":
		return UserFunctionPrincipalInvestigator
	default:
		return UserFunctionOther
	}
}

// parseUserStatus interprets PDBv1 user status string into corresponding `UserStatus`.
func parseUserStatus(s string) UserStatus {
	switch s {
	case "checked in":
		return UserStatusCheckedIn
	case "tentative":
		return UserStatusTentative
	case "checked out":
		return UserStatusCheckedOut
	case "checked out extended":
		return UserStatusCheckedOutExtended
	default:
		return UserStatusUnknown
	}
}
