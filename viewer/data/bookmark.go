package data

import (
	"fmt"
	"time"
	"../config"
)

type Bookmark struct {
	Time float64
	Text string
}

func (self *Bookmark) GetLabel(config *config.Config, logStart float64) string {
	var timePos float64
	if config.Bookmarks.Absolute {
		timePos = self.Time
	} else {
		timePos = self.Time - logStart
	}

	timeText := time.Unix(int64(timePos), 0).Format(config.Bookmarks.Format)
	return fmt.Sprintf("%s: %s", timeText, self.Text)
}
