package graphql

import (
	"strings"
	"time"
)

// func MarshalDate(v *time.Time) ([]byte, error) {
// 	v.Date()
// }

func UnmarshalDate(b []byte, v *time.Time) error {

	// NOTE: this parsing will construct time object in UTC

	t, err := time.Parse("2006-01-02", strings.ReplaceAll(string(b), `"`, ``))

	if err != nil {
		return err
	}

	*v = t
	return nil
}
