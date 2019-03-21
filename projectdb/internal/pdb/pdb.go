// Package pdb provides functions for interacting with the database of the project database.
package pdb

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Donders-Institute/tg-toolset-golang/projectdb/internal/acl"
	log "github.com/sirupsen/logrus"
)

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"source": "db"})
}

// Config defines SQL database connection parameters.
type Config struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
}

// RoleAction defines the data structure of a role action.
type RoleAction struct {
	pid        string
	uid        string
	role       string
	action     string
	quota      int8
	createTime time.Time
}

// User defines the data structure of a user in the project database.
type User struct {
	ID         string
	Firstname  string
	Middlename string
	Lastname   string
	Email      string
}

// SelectPendingRoleMap retrieves pending ACL actions in the project database to be implemented
// on the file system of the project storage.
func SelectPendingRoleMap(db *sql.DB) (map[string][]RoleAction, error) {

	// internal map of role-actions
	roleActionMap := map[string]RoleAction{}

	projectRoleActionMap := map[string][]RoleAction{}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("PDB not connected")
	}

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
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			uid    string
			pid    string
			role   string
			action string
			ctime  time.Time
			quota  int8
		)
		if err := rows.Scan(&uid, &pid, &role, &ctime, &action, &quota); err != nil {
			return nil, err
		}

		// there is already a pending role action on pid+uid, check which one we should consider
		if a, ok := roleActionMap[pid+uid]; ok {
			// compare the existing action 'a' with the current one,
			// and pick up the one created later.
			if ctime.After(a.createTime) {
				roleActionMap[pid+uid] = RoleAction{
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
		roleActionMap[pid+uid] = RoleAction{
			pid:        pid,
			uid:        uid,
			role:       role,
			action:     action,
			quota:      quota,
			createTime: ctime,
		}

		logger.Debug(fmt.Sprintf("%s user %s to role %s in project %s\n", action, uid, role, pid))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// summarise roleActionMap into projectRoleActionMap
	for _, a := range roleActionMap {
		if _, ok := projectRoleActionMap[a.pid]; !ok {
			projectRoleActionMap[a.pid] = []RoleAction{}
		}
		projectRoleActionMap[a.pid] = append(projectRoleActionMap[a.pid], a)
	}

	return projectRoleActionMap, nil
}

// UpdateProjectRoles updates the registry of the data-access roles of the given project
// in the project database, according to the roles provided as a acl.RoleMap.
func UpdateProjectRoles(db *sql.DB, project string, roles acl.RoleMap) error {

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
	setStmt, err = tx.Prepare("INSERT INTO acls (project, user, projectRole) VALUES (?,?,?)")
	if err != nil {
		return err
	}
	for r, users := range roles {
		for _, u := range users {
			// check if the user in question is available in the project database.
			if _, err := SelectUser(db, u); err != nil {
				// ignore user cannot be found in the project database.
				log.Warnf("cannot found users in pdb: %s, reason: %+v", u, err)
				continue
			}
			log.Debugf("Updating project %s, %s: %s", project, r, u)
			_, err := setStmt.Exec(project, u, fmt.Sprintf("%s", r))
			if err != nil {
				return err
			}
		}
	}

	return err
}

// SelectUser gets the user identified by the given uid in the project database.
// It returns the pointer to the user data represented in the User data structure.
func SelectUser(db *sql.DB, uid string) (*User, error) {

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("PDB not connected: %+v", err)
	}

	query := `
	SELECT
		firstName,middleName,lastName,email
	FROM
		users
	WHERE
		id = ?
	`

	var (
		firstname  string
		middlename string
		lastname   string
		email      string
	)

	if err := db.QueryRow(query, uid).Scan(&firstname, &middlename, &lastname, &email); err != nil {
		return nil, err
	}

	user := User{
		ID:         uid,
		Firstname:  firstname,
		Middlename: middlename,
		Lastname:   lastname,
		Email:      email,
	}

	return &user, nil
}
