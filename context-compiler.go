package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	//"runtime/pprof"
	"github.com/mxk/go-sqlite/sqlite3"
	//ctx "github.com/shish/context-apis/go/context"
	"./common"
	"./compiler"
)

type Thread struct {
	id    int
	name  string
	stack []compiler.LogEvent
	lock  *compiler.LogEvent
}

func set_status(text string) {
	fmt.Printf("%s\n", text)
}

func createTables(db *sqlite3.Conn) {
	db.Exec(`
        CREATE TABLE IF NOT EXISTS events(
            id integer primary key,
            thread_id integer not null,
            start_location text not null,   end_location text,
            start_time float not null,      end_time float,
            start_type char(5) not null,    end_type char(5),
            start_text text,                end_text text
        );
    `)
	db.Exec(`
        CREATE TABLE IF NOT EXISTS threads(
            id integer not null,
            node varchar(32) not null,
            process integer not null,
            thread varchar(32) not null
        );
    `)
}

func progressFile(logFile string, lines chan string) {
	fp, err := os.Open(logFile)
	if err != nil {
		log.Fatal(err)
	}
	f_size, err := fp.Seek(0, os.SEEK_END)
	fp.Seek(0, os.SEEK_SET)
	timestamp := time.Unix(0, 0)
	scanner := bufio.NewScanner(fp)
	for n := 0; scanner.Scan(); n++ {
		line := scanner.Text()
		lines <- line
		if n%10000 == 0 {
			time_taken := time.Since(timestamp).Seconds()
			f_pos, _ := fp.Seek(0, os.SEEK_CUR)
			fmt.Printf("Imported %d events (%d%%, %d/s)\n", n, f_pos*100.0/f_size, int(1000.0/time_taken))
			timestamp = time.Now()
		}
	}
	fp.Close()
	close(lines)
}

func getBounds(logFile string) (float64, float64) {
	buf := make([]byte, 1024)

	fp, err := os.Open(logFile)
	if err != nil {
		log.Fatal(err)
	}

	n, err := fp.Read(buf)
	for pos := 0; pos < n; pos++ {
		if buf[pos] == ' ' {
			buf = buf[:pos]
			break
		}
	}
	first, _ := strconv.ParseFloat(string(buf), 64)

	fp.Seek(-1024, os.SEEK_END)

	buf = make([]byte, 1024)
	n, err = fp.Read(buf)
	var newline, space int
	for pos := n - 1; pos >= 0; pos-- {
		if buf[pos] == ' ' {
			space = pos
		}
		if buf[pos] == '\n' && space > 0 {
			newline = pos + 1
			buf = buf[newline:space]
			break
		}
	}
	last, _ := strconv.ParseFloat(string(buf), 64)

	fp.Close()

	return first, last
}

func compileLog(logFile string, databaseFile string) {
	os.Remove(databaseFile) // ignore errors

	db, _ := sqlite3.Open(databaseFile)
	db.Begin()
	createTables(db)

	var threads []Thread
	var summary []int = make([]int, 1000)
	thread_name_to_id := make(map[string]int)
	thread_count := 0
	/*
		    query, _ := db.Query("SELECT node, process, thread FROM threads ORDER BY id")
			for {
				err := query.Next()
				if err == io.EOF {
					break
				}
				var node, process, thread string
				query.Scan(node, process, thread)
				//thread_names = append(thread_names, )
			}
	*/

	sqlInsertBookmark, _ := db.Prepare(`
        INSERT INTO events(thread_id, start_location, start_time, start_type, start_text, end_time)
        VALUES(?, ?, ?, ?, ?, ?)
    `)
	sqlInsertEvent, _ := db.Prepare(`
        INSERT INTO events(
            thread_id,
            start_location, start_time, start_type, start_text,
            end_location, end_time, end_type, end_text
        )
        VALUES(
            ?,
            ?, ?, ?, ?,
            ?, ?, ?, ?
        )
    `)

	firstEventStart, lastEventEnd := getBounds(logFile)
	boundsLength := lastEventEnd - firstEventStart

	lines := make(chan string)
	go progressFile(logFile, lines)
	for line := range lines {
		e := compiler.LogEvent{}
		e.FromLine(line)

		summary[int((e.Timestamp-firstEventStart)/boundsLength*float64(len(summary)-1))]++

		thread_name := e.ThreadID()
		_, exists := thread_name_to_id[thread_name]
		if !exists {
			threads = append(threads, Thread{thread_count, thread_name, make([]compiler.LogEvent, 0), nil})
			thread_name_to_id[thread_name] = thread_count
			thread_count += 1
		}
		thread := &threads[thread_name_to_id[thread_name]]

		switch {
		case e.Type == "START":
			thread.stack = append(thread.stack, e)

		case e.Type == "ENDOK" || e.Type == "ENDER":
			current_depth := len(thread.stack)
			if current_depth > 0 {
				var s compiler.LogEvent
				s, thread.stack = thread.stack[current_depth-1], thread.stack[:current_depth-1]
				sqlInsertEvent.Exec(
					thread.id,
					s.Location, s.Timestamp, s.Type, s.Text,
					e.Location, e.Timestamp, e.Type, e.Text)
			}

		case e.Type == "BMARK":
			sqlInsertBookmark.Exec(
				thread.id, e.Location, e.Timestamp, e.Type, e.Text, e.Timestamp)

		// begin blocking wait for lock
		case e.Type == "LOCKW":
			thread.lock = &e

		// end blocking wait (if there is one) and acquire lock
		case e.Type == "LOCKA":
			if thread.lock != nil {
				s := thread.lock
				sqlInsertEvent.Exec(
					thread.id,
					s.Location, s.Timestamp, s.Type, s.Text,
					e.Location, e.Timestamp, e.Type, e.Text)
				thread.lock = nil
			}
			thread.lock = &e

		// release the lock which was acquired
		case e.Type == "LOCKR":
			if thread.lock != nil {
				s := thread.lock
				sqlInsertEvent.Exec(
					thread.id,
					s.Location, s.Timestamp, s.Type, s.Text,
					e.Location, e.Timestamp, e.Type, e.Text)
				thread.lock = nil
			}
		}
	}

	for idx, thr := range threads {
		parts := strings.Split(thr.name, " ")
		db.Exec(`
            INSERT INTO threads(id, node, process, thread)
            VALUES(?, ?, ?, ?)
        `, idx, parts[0], parts[1], parts[2])
	}

	set_status("Writing summary...")

	db.Exec(`
        CREATE TABLE IF NOT EXISTS summary(
            id integer not null,
			events integer not null
        );
    `)
	for idx, events := range summary {
		db.Exec(`
            INSERT INTO summary(id, events)
            VALUES(?, ?)
        `, idx, events)
	}

	set_status("Indexing bookmarks...")

	db.Exec(`
        CREATE INDEX IF NOT EXISTS idx_start_type_time ON events(start_type, start_time)
    `) // searching for bookmarks

	set_status("Indexing events...")

	db.Exec(`
        CREATE VIRTUAL TABLE events_index USING rtree(id, start_time, end_time)
    `)
	db.Exec(`
        INSERT INTO events_index
        SELECT id, start_time-?, end_time-?
        FROM events
	`, firstEventStart, firstEventStart)
	// WHERE start_time IS NOT NULL AND end_time IS NOT NULL

	set_status("Writing settings...")

	db.Exec(`
        CREATE TABLE IF NOT EXISTS settings(
            version integer not null,
            start_time float not null,
            end_time float not null
        );
    `)
	db.Exec(`
		INSERT INTO settings(version, start_time, end_time)
		VALUES(?, ?, ?)
	`, common.DB_VER, firstEventStart, lastEventEnd)

	db.Commit()
	db.Close()
}

func main() {
	var logFile, databaseFile string

	//f, _ := os.Create("profile.dat")
	//pprof.StartCPUProfile(f)
	//defer pprof.StopCPUProfile()

	if len(os.Args) >= 2 {
		logFile = os.Args[1]
	} else {
		log.Fatal("Missing input filename")
	}

	if len(os.Args) == 3 {
		databaseFile = os.Args[2]
	} else {
		databaseFile = strings.Replace(logFile, ".ctxt", ".cbin", -1)
	}

	compileLog(logFile, databaseFile)
}
