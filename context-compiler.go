package main

import (
	"os"
	"time"
	"fmt"
	"log"
	"bufio"
	"strings"
	"strconv"
	//"runtime/pprof"
	"code.google.com/p/go-sqlite/go1/sqlite3"
	//ctx "github.com/shish/context-apis/go/context"
)


type LogEvent struct {
	Timestamp float64
	Node string
	Process int64
	Thread string
	Type string
	Location string
	Text string
}

func (self *LogEvent) FromLine(line string) {
	// TODO regex?
/*
	n, _ := fmt.Sscanf(line, "%f %s %d %s %s %s %s\n",
		&self.Timestamp, &self.Node, &self.Process, &self.Thread,
		&self.Type, &self.Location, &self.Text)
	if n < 6 {
		fmt.Printf("Error parsing %s\n", line)
	}
*/

	parts := strings.SplitN(strings.Trim(line, "\n"), " ", 7)
	//fmt.Printf("parts: %d %s\n", len(parts), parts)
    self.Timestamp, _ = strconv.ParseFloat(parts[0], 64)
	self.Node = parts[1]
    self.Process, _ = strconv.ParseInt(parts[2], 10, 32)
	self.Thread = parts[3]
	self.Type = parts[4]
	self.Location = parts[5]
	self.Text = parts[6]
}

func (self *LogEvent) ThreadID() string {
	return fmt.Sprintf("%s %d %s", self.Node, self.Process, self.Thread)
}

func (self *LogEvent) EventStr() string {
    return fmt.Sprintf("%s %s:%s", self.Location, self.Type, self.Text)
}

func (self *LogEvent) ToString() string {
    return self.ThreadID() + " " + self.EventStr()
}


type Thread struct {
	id int
	name string
	stack []LogEvent
	lock *LogEvent
}


func set_status(text string) {
	fmt.Printf("%s\n", text)
}


func createTables(db *sqlite3.Conn) {
    db.Exec(`
        CREATE TABLE IF NOT EXISTS cbtv_events(
            id integer primary key,
            thread_id integer not null,
            start_location text not null,   end_location text,
            start_time float not null,      end_time float,
            start_type char(5) not null,    end_type char(5),
            start_text text,                end_text text
        );
    `)
    db.Exec(`
        CREATE TABLE IF NOT EXISTS cbtv_threads(
            id integer not null,
            node varchar(32) not null,
            process integer not null,
            thread varchar(32) not null
        );
    `)
}


func progressFile(logFile string, lines chan string) {
    fp, err := os.Open(logFile)
	if err != nil {log.Fatal(err)}
    f_size, err := fp.Seek(0, os.SEEK_END)
    fp.Seek(0, os.SEEK_SET)
    timestamp := time.Unix(0, 0)
	scanner := bufio.NewScanner(fp)
    for n := 0 ; scanner.Scan() ; n++ {
		line := scanner.Text()
		lines <- line
        if n % 10000 == 0 {
            time_taken := time.Since(timestamp).Seconds()
			f_pos, _ := fp.Seek(0, os.SEEK_CUR)
            fmt.Printf("Imported %d events (%d%%, %d/s)\n", n, f_pos * 100.0 / f_size, int(1000.0/time_taken))
            timestamp = time.Now()
		}
	}
    fp.Close()
	close(lines)
}


func compileLog(logFile string, databaseFile string) {
	os.Remove(databaseFile)  // ignore errors

    db, _ := sqlite3.Open(databaseFile)
	db.Begin()
	createTables(db)

	var first_event_start float64
	var threads []Thread
	thread_name_to_id := make(map[string]int)
	thread_count := 0
	/*
    query, _ := db.Query("SELECT node, process, thread FROM cbtv_threads ORDER BY id")
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
        INSERT INTO cbtv_events(thread_id, start_location, start_time, start_type, start_text)
        VALUES(?, ?, ?, ?, ?)
    `)
	sqlInsertEvent, _ := db.Prepare(`
        INSERT INTO cbtv_events(
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

	lines := make(chan string)
	go progressFile(logFile, lines)
    for line := range lines {
        e := LogEvent{}
		e.FromLine(line)

        thread_name := e.ThreadID()
		_, exists := thread_name_to_id[thread_name]
        if !exists {
			threads = append(threads, Thread{thread_count, thread_name, make([]LogEvent, 0), nil})
			thread_name_to_id[thread_name] = thread_count
			thread_count += 1
		}
        thread := &threads[thread_name_to_id[thread_name]]

        if first_event_start == 0.0 {
            first_event_start = e.Timestamp
		}

		switch {
			case e.Type == "START":
				thread.stack = append(thread.stack, e)

			case e.Type == "ENDOK" || e.Type == "ENDER":
				current_depth := len(thread.stack)
				if current_depth > 0 {
					var s LogEvent
					s, thread.stack = thread.stack[current_depth-1], thread.stack[:current_depth-1]
					sqlInsertEvent.Exec(
						thread.id,
						s.Location, s.Timestamp, s.Type, s.Text,
						e.Location, e.Timestamp, e.Type, e.Text)
				}

			case e.Type == "BMARK":
				sqlInsertBookmark.Exec(
					thread.id, e.Location, e.Timestamp, e.Type, e.Text)

			// begin blocking wait for lock
			case e.Type == "LOCKW":
				thread.lock = &e

			// end blocking wait (if there is one) and aquire lock
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

			// release the lock which was aquired
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

    db.Exec("DELETE FROM cbtv_threads")
    for idx, thr := range threads {
		parts := strings.Split(thr.name, " ")
        db.Exec(`
            INSERT INTO cbtv_threads(id, node, process, thread)
            VALUES(?, ?, ?, ?)
        `, idx, parts[0], parts[1], parts[2])
	}

    set_status("Indexing bookmarks...")

    db.Exec(`
        CREATE INDEX IF NOT EXISTS idx_start_type_time ON cbtv_events(start_type, start_time)
    `)  // searching for bookmarks

    set_status("Indexing events...")

    db.Exec(`
        CREATE VIRTUAL TABLE cbtv_events_index USING rtree(id, start_time, end_time)
    `)
    db.Exec(`
        INSERT INTO cbtv_events_index
        SELECT id, start_time-?, end_time-?
        FROM cbtv_events
        WHERE start_time IS NOT NULL AND end_time IS NOT NULL
	`, first_event_start, first_event_start)

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

