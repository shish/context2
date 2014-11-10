package event

import (
	"fmt"
)

/*
   #########################################################################
   # Event
   #########################################################################
*/
type Event struct {
	ID            int
	ThreadID      int // index into the list of all threads
	ThreadIndex   int // index into the list of currently visible threads
	StartLocation string
	EndLocation   string
	StartTime     float64
	EndTime       float64
	StartType     string
	EndType       string
	StartText     string
	EndText       string
	count         int
	Depth         int
}

func (self *Event) NewEvent() {
	self.count = 1
	self.Depth = 0
}

func (self *Event) CanMerge(other Event, threshold float64) bool {
	return (other.Depth == self.Depth &&
		other.ThreadID == self.ThreadID &&
		other.StartTime-self.EndTime < 0.01 &&
		other.Length() < threshold &&
		other.StartText == self.StartText)
}

func (self *Event) Merge(other Event) {
	self.EndTime = other.EndTime
	self.count += 1
}

func (self *Event) Text() string {
	var text string

	if self.StartText == self.EndText || self.EndText == "" {
		text = self.StartText
	} else {
		text = self.StartText + "\n" + self.EndText
	}

	if self.count > 1 {
		text = fmt.Sprintf("%d x %s", self.count, text)
	}

	return text
}

func (self *Event) Tip(offsetTime float64) string {
	var pre, loc string

	if len(self.StartLocation) > 50 {
		pre = "..."
		loc = self.StartLocation[len(self.StartLocation)-50:]
	} else {
		pre = ""
		loc = self.StartLocation
	}

	return fmt.Sprintf("%.0fms @%.0fms: %s%s",
		(self.EndTime-self.StartTime)*1000,
		(self.StartTime-offsetTime)*1000,
		pre, loc)
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
