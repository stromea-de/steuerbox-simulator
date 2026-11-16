package inspection

import "time"

// ReportRevision describes a correction of assignment data, never a new test.
// The original report, test times and measurements remain available unchanged.
type ReportRevision struct {
	Number       int       `json:"number"`
	RootID       string    `json:"rootId"`
	PreviousID   string    `json:"previousId"`
	PreviousDate time.Time `json:"previousDate"`
	Created      time.Time `json:"created"`
}

func (r Report) VersionNumber() int {
	if r.Revision != nil && r.Revision.Number > 1 {
		return r.Revision.Number
	}
	return 1
}
