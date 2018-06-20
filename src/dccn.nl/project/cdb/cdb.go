// Package cdb provides functions for interacting with the database of the lab booking calendar.
package cdb

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"

	"dccn.nl/project/pdb"

	log "github.com/sirupsen/logrus"
)

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"source": "cdb"})
}

// LabBooking defines the data structure of a booking event in the lab calendar.
type LabBooking struct {
	// Project is the id of the project to which the experiment belongs.
	Project string
	// Subject is the subject id of the participant.
	Subject string
	// Session is the session id of the participant.
	Session string
	// Modality is the experiment modality name.
	Modality string
	// Operator is the user operating the experiment.
	Operator pdb.PdbUser
	// ProjectTitle is the title of the project.
	ProjectTitle string
	// StartTime is the time the experiment starts.
	StartTime time.Time
}

// Lab defines an enumerator for the lab categories.
type Lab int

const (
	// MEG is a lab category of the MEG labs.
	MEG Lab = iota
	// MRI is a lab category of the MRI labs.
	MRI
)

// SelectLabBookings queries the booking events of a given lab on a given date.
func SelectLabBookings(db *sql.DB, lab Lab, date string) ([]LabBooking, error) {

	if err := db.Ping(); err != nil {
		return nil, errors.New(fmt.Sprintf("PDB not connected: %+v", err))
	}

	query := `
	SELECT
		a.id,a.project_id,a.subj_ses,a.start_time,a.user_id,b.projectName,c.description
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

	// slice of LabBooking events
	bookings := []LabBooking{}

	// regular expression for matching lab description
	var labPat *regexp.Regexp
	switch lab {
	case MEG:
		labPat = regexp.MustCompile(".*(MEG).*")
	case MRI:
		labPat = regexp.MustCompile(".*(SKYRA|PRASMA(FIT)).*")
	default:
		return nil, errors.New("Unknown lab category")
	}

	// regular expression for spliting subject and session identifiers
	subjsesSpliter := regexp.MustCompile("\\s*(-)\\s*")

	// loop over results of the query
	for rows.Next() {
		var (
			id       string
			pid      string
			subj_ses string
			stime    []uint8
			uid      string
			pname    string
			labdesc  string
		)

		err := rows.Scan(&id, &pid, &subj_ses, &stime, &uid, &pname, &labdesc)
		if err != nil {
			return nil, err
		}

		if m := labPat.FindStringSubmatch(labdesc); len(m) >= 2 {
			var (
				subj string
				sess string
			)
			if dss := subjsesSpliter.Split(subj_ses, -1); len(dss) < 2 {
				subj = dss[0]
				sess = "1"
			} else {
				subj = dss[0]
				sess = dss[1]
			}

			tstr := fmt.Sprintf("%sT%s", date, stime)
			t, err := time.Parse(time.RFC3339[:19], tstr)
			if err != nil {
				log.Errorf("cannot parse time: %s", tstr)
				continue
			}

			pdbUser, err := pdb.SelectPdbUser(db, uid)
			if err != nil {
				log.Errorf("cannot find user in PDB: %s", uid)
				continue
			}

			bookings = append(bookings, LabBooking{
				Project:      pid,
				Subject:      subj,
				Session:      sess,
				Modality:     m[1],
				ProjectTitle: pname,
				Operator:     *pdbUser,
				StartTime:    t,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return bookings, nil
}
