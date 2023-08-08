package dicom_worklist

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"

	log "github.com/dccn-tg/tg-toolset-golang/pkg/logger"
	ipdb "github.com/dccn-tg/tg-toolset-golang/project/pkg/pdb"
)

var (
	cmdDump2dcm string = "dump2dcm" // command for dump2dcm
	dryRun      bool
	date        string                                     // date format YYYY-MM-DD
	scanners    = []string{"prisma", "prismafit", "skyra"} // valid scanner names
	store       string                                     // path of the worklist (for both .dump and .wl files) store
)

// data structure for a DICOM worklist
type worklistData struct {
	EventID      string
	Date         string
	Time         string
	PatientID    string
	PatientName  string
	SessionID    string
	SessionTitle string
	Physician    string
	ProjectTitle string
	ModalityAE   string
}

// template for human-readable DICOM worklist
const worklistTemplate = `(0010,0010) PN  [{{.PatientName}}]
(0010,0020) LO  [{{.PatientID}}]
(0020,000d) UI  [{{.EventID}}]
(0032,1032) PN  [{{.Physician}}]
(0008,0090) PN  [{{.Physician}}]
(0032,1060) LO  [{{.ProjectTitle}}]
(0040,1001) SH  [{{.SessionID}}]
(0040,0100) SQ
(fffe,e000) -
(0008,0060) CS  [MR]
(0040,0001) AE  [{{.ModalityAE}}]
(0040,0002) DA  [{{.Date}}]
(0040,0003) TM  [{{.Time}}]
(0040,0009) SH  [{{.SessionID}}]
(0040,0010) SH  [{{.ModalityAE}}]
(0040,0011) SH  [DCCN]
(0040,0007) LO  [{{.SessionTitle}}]
(0040,0008) SQ
(fffe,e0dd) - 
(fffe,e00d) -
(fffe,e0dd) -
`

// composeWorklist composes a human-readable worklist using the data provided.
// provided.
func composeWorklist(data worklistData) (string, error) {
	var buf bytes.Buffer
	t := template.Must(template.New("worklist").Parse(worklistTemplate))
	err := t.Execute(&buf, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func init() {

	cwd, _ := os.Getwd()

	generateCmd.PersistentFlags().BoolVarP(&dryRun, "list", "l", false,
		"list the generated worklist only (i.e. not saving the worklist to files in the DICOM format)")

	generateCmd.Flags().StringVarP(&date, "date", "d", time.Now().Format(time.RFC3339[:10]),
		"specify data acquisition date in format of YYYY-MM-DD")

	generateCmd.Flags().StringVarP(&store, "path", "p", cwd,
		"specify the path of the worklist store")

	rootCmd.AddCommand(generateCmd)
}

var generateCmd = &cobra.Command{
	Use:   fmt.Sprintf("generate {%s [...]}", strings.Join(scanners, "|")),
	Short: "Generate DICOM worklist from Lab bookings of the DCCN's MR scanners",
	Long: fmt.Sprintf(`Generate DICOM worklist from Lab bookings of the DCCN's MR scanners.
	
Use the argument to specify one or multiple available scanners: %s
`, strings.Join(scanners, ", ")),
	ValidArgs: scanners,
	Args:      cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {

		pdb := loadPdb()

		bookings, err := pdb.GetLabBookingsForWorklist(ipdb.MRI, date)
		if err != nil {
			log.Errorf("cannot retrieve labbookings, reason: %+v", err)
			os.Exit(100)
		}

		// internal function to check if a booking is valid for creating worklist
		validBooking := func(booking *ipdb.LabBooking) bool {
			_scanner := strings.ToLower(booking.Lab)
			for _, s := range args {
				if _scanner == s {
					return true
				}
			}
			return false
		}

		for _, booking := range bookings {
			if !validBooking(booking) {
				// skip invalid booking
				continue
			}

			// internal format of session id
			var _sessId string
			if _id, err := strconv.Atoi(booking.Session); err != nil {
				_sessId = booking.Session
			} else {
				_sessId = fmt.Sprintf("%02d", _id)
			}

			// internal format for date and time
			_date := strings.ReplaceAll(booking.StartTime.Format(time.RFC3339[:10]), "-", "")
			_time := fmt.Sprintf("%02d%02d%02d", booking.StartTime.Hour(), booking.StartTime.Minute(), booking.StartTime.Second())

			// internal format for patient id
			var _pid string
			switch {
			case booking.Subject == "" || strings.HasPrefix(booking.Subject, "Undefined"): // undefined
				// format `{project}_sub-{YYYYMMDD}T{hhmmss}`
				_pid = fmt.Sprintf("%s_sub-%sT%s", booking.Project, _date, _time)
			case strings.HasPrefix(booking.Subject, "x") || strings.HasPrefix(booking.Subject, "X"): // extra
				// format: `{project}_sub-x{NN}`
				var _subId string
				if _id, err := strconv.Atoi(booking.Subject[1:]); err != nil {
					_subId = booking.Subject[1:]
				} else {
					_subId = fmt.Sprintf("%03d", _id)
				}
				_pid = fmt.Sprintf("%s_sub-x%s", booking.Project, _subId)
			default:
				// default format: `{project}_sub-{NN}`
				var _subId string
				if _id, err := strconv.Atoi(booking.Subject); err != nil {
					_subId = booking.Subject[1:]
				} else {
					_subId = fmt.Sprintf("%03d", _id)
				}
				_pid = fmt.Sprintf("%s_sub-%s", booking.Project, _subId)
			}

			_data := worklistData{
				EventID:      fmt.Sprintf("%s%s", _date, _time),
				Date:         _date,
				Time:         _time,
				PatientID:    _pid,
				PatientName:  fmt.Sprintf("%s_ses-%s", _pid, _sessId),
				SessionID:    fmt.Sprintf("ses-mri%s", _sessId),
				SessionTitle: fmt.Sprintf("MR session %s", _sessId),
				ProjectTitle: booking.ProjectTitle,
				Physician:    fmt.Sprintf("%s %s", booking.Operator.Firstname, booking.Operator.Lastname),
				ModalityAE:   strings.ToUpper(booking.Lab),
			}

			wl, err := composeWorklist(_data)

			if err != nil {
				log.Errorf("[%s:%s] cannot create worklist for booking: %s", _data.ModalityAE, _data.EventID, err)
				continue
			}

			// dryrun
			if dryRun {
				fmt.Printf("%s\n", wl)
				continue
			}

			// dump to file
			_fdump := filepath.Join(store, fmt.Sprintf("%s_%s.dump", _data.ModalityAE, _data.EventID))
			if err := os.WriteFile(_fdump, []byte(wl), 0644); err != nil {
				log.Errorf("[%s:%s] cannot dump worklist: %s", _data.ModalityAE, _data.EventID, err)
				continue
			}

			// convert to DICOM format, using dcmtk's `dump2dcm` command
			_fdcm := filepath.Join(store, fmt.Sprintf("%s_%s.wl", _data.ModalityAE, _data.EventID))
			cmd := exec.Command(cmdDump2dcm, _fdump, _fdcm)

			if err := cmd.Run(); err != nil {
				log.Errorf("[%s:%s] cannot convert DICOM format: %s", _data.ModalityAE, _data.EventID, err)
				continue
			}
		}

		return nil
	},
}
