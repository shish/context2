package data

import (
	"bufio"
	"github.com/mxk/go-sqlite/sqlite3"
	"fmt"
	"github.com/conformal/gotk3/gtk"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"../../common"
	"../config"
	"../event"
)

type Data struct {
	conn      *sqlite3.Conn
	config    config.Config
	Data      []event.Event
	Bookmarks *gtk.ListStore
	Summary   []int
	Threads   []string
	LogStart  float64
	LogEnd    float64
}

func splitExt(path string) (root, ext string) {
	ext = filepath.Ext(path)
	root = path[:len(path)-len(ext)]
	return
}

func VersionCheck(databaseFile string) bool {
	log.Println("Checking database version")
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
	if version != common.DB_VER {
		fmt.Printf("Incompatible binary version: %d != %d\n", version, common.DB_VER)
		return false
	}

	return true
}

func (self *Data) LoadFile(givenFile string, setStatus func(string), config config.Config) (string, error) {
	log.Println("Loading: file", givenFile)

	self.config = config

	path, _ := splitExt(givenFile)
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
			log.Println("Recompiling")
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

	if self.conn != nil {
		self.conn.Close()
		self.conn = nil
	}
	self.conn, _ = sqlite3.Open(databaseFile)

	self.Data = []event.Event{} // don't load the bulk of the data yet
	self.LoadSettings()
	self.LoadBookmarks()
	self.LoadSummary()
	self.LoadThreads()

	return databaseFile, nil
}

func (self *Data) LoadEvents(renderStart, renderLen, coalesce, cutoff float64, setStatus func(string)) {
	log.Println("Loading: events")

	setStatus("Loading...")
	defer setStatus("")
	s := renderStart
	e := renderStart + renderLen
	self.Data = []event.Event{} // free memory
	threadLevelEnds := make([][]event.Event, len(self.Threads))

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
		AND (
			(end_time - start_time) >= ? OR
			start_type = "BMARK"
		)
		ORDER BY start_time ASC, end_time DESC
	`
	for query, err := self.conn.Query(sql, s-self.LogStart, e-self.LogStart, cutoff); err == nil; err = query.Next() {
		var evt event.Event
		evt.NewEvent(query)
		evt.ThreadIndex := evt.ThreadID // TODO: index into currently-active Threads, not all Threads

		if evt.StartType == "START" {
			var prevEventAtLevel *event.Event
			for {
				endIdx := len(threadLevelEnds[evt.ThreadIndex]) - 1
				if endIdx < 0 || threadLevelEnds[evt.ThreadIndex][endIdx].EndTime > evt.StartTime {
					break
				}
				prevEventAtLevel = &threadLevelEnds[evt.ThreadIndex][endIdx]
				threadLevelEnds[evt.ThreadIndex] = threadLevelEnds[evt.ThreadIndex][:endIdx]
			}
			evt.Depth = len(threadLevelEnds[evt.ThreadIndex])

			if coalesce > 0.0 &&
				prevEventAtLevel != nil &&
				prevEventAtLevel.CanMerge(evt, coalesce) {
				prevEventAtLevel.Merge(evt)
				//log.Printf("%.2f %.2f\n", prevEventAtLevel.StartTime, prevEventAtLevel.EndTime)
				threadLevelEnds[evt.ThreadIndex] = append(threadLevelEnds[evt.ThreadIndex], *prevEventAtLevel)
			} else {
				threadLevelEnds[evt.ThreadIndex] = append(threadLevelEnds[evt.ThreadIndex], evt)
				self.Data = append(self.Data, evt)
			}
		} else {
			self.Data = append(self.Data, evt)
		}
	}

	setStatus("Sorting events")
	sort.Sort(event.ByType(self.Data))

	log.Println("Loading: done")
}

func (self *Data) LoadBookmarks() {
	log.Println("Loading: bookmarks")

	n := 0
	self.Bookmarks.Clear()

	sql := "SELECT start_time, start_text, end_text FROM events WHERE start_type = 'BMARK' ORDER BY start_time"
	for query, err := self.conn.Query(sql); err == nil; err = query.Next() {
		if n % 1000 == 0 {
			log.Printf("Loaded %d bookmarks\n", n)
		}
		n++
		var startTime float64
		var startText, endText string
		query.Scan(&startTime, &startText, &endText)

		var timePos float64
		if self.config.Bookmarks.Absolute {
			timePos = startTime
		} else {
			timePos = startTime - self.LogStart
		}

		timeText := time.Unix(int64(timePos), 0).Format(self.config.Bookmarks.Format)
		text := fmt.Sprintf("%s: %s", timeText, startText)

		itemPtr := self.Bookmarks.Append()
		self.Bookmarks.Set(itemPtr, []int{0, 1}, []interface{}{startTime, text})
	}
}

func (self *Data) LoadSettings() {
	log.Println("Loading: settings")

	sql := "SELECT start_time, end_time FROM settings"
	for query, err := self.conn.Query(sql); err == nil; err = query.Next() {
		query.Scan(&self.LogStart, &self.LogEnd)
	}
}

func (self *Data) LoadThreads() {
	log.Println("Loading: threads")

	self.Threads = make([]string, 0, 10)

	sql := "SELECT node, process, thread FROM threads ORDER BY id"
	for query, err := self.conn.Query(sql); err == nil; err = query.Next() {
		var node, process, thread string
		query.Scan(&node, &process, &thread)
		self.Threads = append(self.Threads, fmt.Sprintf("%s-%s-%s", node, process, thread))
	}
}

func (self *Data) LoadSummary() {
	log.Println("Loading: summary")

	self.Summary = make([]int, 0, 1000)

	sql := "SELECT events FROM summary ORDER BY id"
	for query, err := self.conn.Query(sql); err == nil; err = query.Next() {
		var val int
		query.Scan(&val)
		self.Summary = append(self.Summary, val)
	}
}

func (self *Data) GetEarliestBookmarkAfter(startHint float64) float64 {
	var startTime float64
	sql := "SELECT min(start_time) FROM events WHERE start_time > ? AND start_type = 'BMARK'"
	for query, err := self.conn.Query(sql, startHint); err == nil; err = query.Next() {
		query.Scan(&startTime)
	}
	return startTime
}

func (self *Data) GetLatestBookmarkBefore(endHint float64) float64 {
	var endTime float64
	sql := "SELECT max(start_time) FROM events WHERE start_time < ? AND start_type = 'BMARK'"
	for query, err := self.conn.Query(sql, endHint); err == nil; err = query.Next() {
		query.Scan(&endTime)
	}
	return endTime
}
