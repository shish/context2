package viewer

import (
	"fmt"
	"strings"
	"os"
	"os/exec"
	"bufio"
	"time"
	"code.google.com/p/go-sqlite/go1/sqlite3"
	"github.com/conformal/gotk3/gtk"
)

/*
   #########################################################################
   # Database
   #########################################################################
*/

type Data struct {
	Conn      *sqlite3.Conn
	Data      []Event
	Bookmarks *gtk.ListStore
	Summary   []int
	Threads   []string
	LogStart  float64
	LogEnd    float64
}

func VersionCheck(databaseFile string) bool {
	db, _ := sqlite3.Open(databaseFile)
	defer db.Close()

	query, err := db.Query("SELECT version FROM settings LIMIT 1")
	if err != nil {
		fmt.Printf("Error getting version query: %s\n", err)
		return false
	}

	var version int
	err = query.Scan(&version)
	if err != nil {
		fmt.Printf("Error getting version row: %s\n", err)
		return false
	}
	if version != 1 {
		fmt.Printf("Version too old: %d\n", version)
		return false
	}

	return true
}

func (self *Data) LoadFile(givenFile string, setStatus func(string)) (string, error) {
	/*
	   path, _ext = os.path.splitext(givenFile)
	*/
	path := strings.Split(givenFile, ".")[0]
	logFile := path + ".ctxt"
	databaseFile := path + ".cbin"

	logStat, err := os.Stat(logFile)
	if err != nil {
		return "", err
	}

	// if the user picked a log file, compile it (unless an
	// up-to-date version already exists)
	if givenFile == logFile {
		needsRecompile := false

		databaseStat, err := os.Stat(databaseFile)

		if err != nil {
			needsRecompile = true
			setStatus("Compiled log not found, compiling")
		} else if logStat.ModTime().UnixNano() > databaseStat.ModTime().UnixNano() {
			needsRecompile = true
			setStatus("Compiled log is out of date, recompiling")
		} else if !VersionCheck(databaseFile) {
			needsRecompile = true
			setStatus("Compiled log is from an old version of context, recompiling")
		}

		if needsRecompile {
			compiler := exec.Command("context-compiler", logFile)
			pipe, _ := compiler.StdoutPipe()
			reader := bufio.NewScanner(pipe)
			compiler.Start()

			for reader.Scan() {
				line := reader.Text()
				if line != "" {
					setStatus(strings.Trim(line, "\n\r"))
				} else {
					break
				}
			}
		}
	}

	if self.Conn != nil {
		self.Conn.Close()
		self.Conn = nil
	}
	self.Conn, _ = sqlite3.Open(databaseFile)

	self.Data = []Event{} // don't load the bulk of the data yet
	// self.LoadEvents()
	self.LoadBookmarks()
	self.LoadSummary()
	self.LoadThreads()

	return databaseFile, nil
}

func (self *Data) LoadEvents(renderStart, renderLen, coalesceThreshold float64, renderCutoff int, setStatus func(string)) {
	defer setStatus("")
	s := renderStart
	e := renderStart + renderLen
	threshold := float64(coalesceThreshold) / 1000.0
	self.Data = []Event{} // free memory
	threadLevelEnds := make([][]Event, len(self.Threads))

	/*
			n = 0
		   	func progress() {
		   		n++
		   		setStatus("Loading... (%dk opcodes)" % (self.n * 10))
		   		return 0  // non-zero = cancel query
		   	}
		   	self.c.set_progress_handler(progress, 10000)
		       defer self.c.set_progress_handler(None, 0)
	*/

	sql := `
		SELECT *
		FROM events
		WHERE id IN (SELECT id FROM events_index WHERE end_time > ? AND start_time < ?)
		AND (end_time - start_time) * 1000 >= ?
		ORDER BY start_time ASC, end_time DESC
	`
	for query, err := self.Conn.Query(sql, s-self.LogStart, e-self.LogStart, renderCutoff); err == nil; err = query.Next() {
		var event Event
		event.NewEvent(query)
		thread_idx := event.ThreadID // TODO: index into currently-active Threads, not all Threads

		if event.StartType == "START" {
			var prevEventAtLevel *Event
			for {
				endIdx := len(threadLevelEnds[thread_idx]) - 1
				if endIdx < 0 || threadLevelEnds[thread_idx][endIdx].EndTime > event.StartTime {
					break
				}
				prevEventAtLevel = &threadLevelEnds[thread_idx][endIdx]
				threadLevelEnds[thread_idx] = threadLevelEnds[thread_idx][:endIdx]
			}
			event.Depth = len(threadLevelEnds[thread_idx])

			if threshold > 0.0 &&
				prevEventAtLevel != nil &&
				prevEventAtLevel.CanMerge(event, threshold) {
				prevEventAtLevel.Merge(event)
				threadLevelEnds[thread_idx] = append(threadLevelEnds[thread_idx], *prevEventAtLevel)
			} else {
				threadLevelEnds[thread_idx] = append(threadLevelEnds[thread_idx], event)
				self.Data = append(self.Data, event)
			}
		} else {
			self.Data = append(self.Data, event)
		}
	}
}

func (self *Data) LoadBookmarks() {
	self.Bookmarks.Clear()

	sql := "SELECT start_time, start_text, end_text FROM events WHERE start_type = 'BMARK' ORDER BY start_time"
	for query, err := self.Conn.Query(sql); err == nil; err = query.Next() {
		var startTime float64
		var startText, endText string
		query.Scan(&startTime, &startText, &endText)

		itemPtr := self.Bookmarks.Append()
		text := fmt.Sprintf("%19.19s: %s", time.Unix(int64(startTime), 0), startText)
		self.Bookmarks.Set(itemPtr, []int{0, 1}, []interface{}{startTime, text})
	}
}

func (self *Data) LoadThreads() {
	self.Threads = make([]string, 0, 10)

	sql := "SELECT node, process, thread FROM Threads ORDER BY id"
	for query, err := self.Conn.Query(sql); err == nil; err = query.Next() {
		var node, process, thread string
		query.Scan(&node, &process, &thread)
		self.Threads = append(self.Threads, fmt.Sprintf("%s-%s-%s", node, process, thread))
	}
}

func (self *Data) LoadSummary() {
	self.Summary = make([]int, 0, 1000)

	sql := "SELECT events FROM Summary ORDER BY id"
	for query, err := self.Conn.Query(sql); err == nil; err = query.Next() {
		var val int
		query.Scan(&val)
		self.Summary = append(self.Summary, val)
	}

	// TODO: bake this into the .cbin
	sql = "SELECT min(start_time), max(end_time) FROM events"
	for query, err := self.Conn.Query(sql); err == nil; err = query.Next() {
		query.Scan(&self.LogStart, &self.LogEnd)
		//fmt.Printf("Range: %d:%d", self.LogStart, self.LogEnd)
	}
}
