package event

import (
	"github.com/mxk/go-sqlite/sqlite3"
	"fmt"
)

/*
   #########################################################################
   # Event
   #########################################################################
*/
type Event struct {
	id             int
	ThreadID       int
	startLocation string
	endLocation   string
	StartTime      float64
	EndTime        float64
	StartType      string
	EndType        string
	startText     string
	endText       string
	count          int
	Depth          int
}

func (self *Event) NewEvent(query *sqlite3.Stmt) {
	query.Scan(
		&self.id,
		&self.ThreadID,
		&self.startLocation, &self.endLocation,
		&self.StartTime, &self.EndTime,
		&self.StartType, &self.EndType,
		&self.startText, &self.endText,
	)

	self.count = 1
	self.Depth = 0
}

func (self *Event) CanMerge(other Event, threshold float64) bool {
	return (other.Depth == self.Depth &&
		other.ThreadID == self.ThreadID &&
		other.StartTime-self.EndTime < 0.001 &&
		other.Length() < threshold &&
		other.startText == self.startText)
}

func (self *Event) Merge(other Event) {
	self.EndTime = other.EndTime
	self.count += 1
}

func (self *Event) Text() string {
	var text string

	if self.startText == self.endText || self.endText == "" {
		text = self.startText
	} else {
		text = self.startText + "\n" + self.endText
	}

	if self.count > 1 {
		text = fmt.Sprintf("%d x %s", self.count, text)
	}

	return text
}

func (self *Event) Tip(offsetTime float64) string {
	return fmt.Sprintf("%.0fms @%.0fms: %s",
		   (self.EndTime - self.StartTime) * 1000,
		   (self.StartTime - offsetTime) * 1000,
		   self.startLocation)
}

func (self *Event) Length() float64 {
	return self.EndTime - self.StartTime
}

// for sorting
type ByType []Event
type stringSlice []string

var types = stringSlice{"LOCKW", "LOCKA", "START", "BMARK"}

func (slice stringSlice) pos(value string) int {
	for p, v := range slice {
		if v == value {
			return p
		}
	}
	return -1
}

func (a ByType) Len() int      { return len(a) }
func (a ByType) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByType) Less(i, j int) bool {
	if a[i].StartType == a[j].StartType {
		return a[i].StartTime < a[j].StartTime
	} else {
		return types.pos(a[i].StartType) < types.pos(a[j].StartType)
	}
	return false
}

func CmpEvent(a *Event, b *Event) bool {
	if a == nil && b == nil {
		return true
	} else if a == nil || b == nil {
		return false
	} else {
		return a.StartTime == b.StartTime && a.ThreadID == b.ThreadID
	}
}
