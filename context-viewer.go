package main

import (
	//"bufio"
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"
	//"strconv"
	"strings"
	//"time"
	//"runtime/pprof"
	"code.google.com/p/go-sqlite/go1/sqlite3"
	//ctx "github.com/shish/context-apis/go/context"
	"code.google.com/p/gcfg"
	"github.com/shish/gotk3/cairo"
	"github.com/conformal/gotk3/glib"
	"github.com/conformal/gotk3/gtk"
	//"github.com/conformal/gotk3/pango"
)

const (
	NAME            = "Context"
	VERSION         = "v0.0.0"
	MAX_DEPTH       = 7
	BLOCK_HEIGHT    = 20
	HEADER_HEIGHT   = 20
	SCRUBBER_HEIGHT = 20
	MIN_PPS         = 1
	MAX_PPS         = 5000
	MIN_SEC         = 1
	MAX_SEC         = 600
)

//if VERSION.endswith("-demo"):
//    NAME += ": Non-commercial / Evaluation Version"

//os.environ["PATH"] = os.environ.get("PATH", "") + ":%s" % os.path.dirname(sys.argv[0])

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

type Event struct {
	id             int
	thread_id      int
	start_location string
	end_location   string
	start_time     float64
	end_time       float64
	start_type     string
	end_type       string
	start_text     string
	end_text       string
	count          int
	depth          int
}

func (self *Event) NewEvent(query *sqlite3.Stmt) {
	query.Scan(
		&self.id,
		&self.thread_id,
		&self.start_location, &self.end_location,
		&self.start_time, &self.end_time,
		&self.start_type, &self.end_type,
		&self.start_text, &self.end_text,
	)

	self.count = 1
	self.depth = 0
}

func (self *Event) CanMerge(other Event, threshold float64) bool {
	return (other.depth == self.depth &&
		other.thread_id == self.thread_id &&
		other.start_time-self.end_time < 0.001 &&
		other.Length() < threshold &&
		other.start_text == self.start_text)
}

func (self *Event) Merge(other Event) {
	self.end_time = other.end_time
	self.count += 1
}

func (self *Event) Text() string {
	var text string

	if self.start_text == self.end_text || self.end_text == "" {
		text = self.start_text
	} else {
		text = self.start_text + "\n" + self.end_text
	}

	if self.count > 1 {
		text = fmt.Sprintf("%d x %s", self.count, text)
	}

	return text
}

func (self *Event) Length() float64 {
	return self.end_time - self.start_time
}

type Config struct {
	Gui struct {
		RenderLen         int    `gcfg:"render_len"`
		Scale             int    `gcfg:"scale"`
		RenderCutoff      int    `gcfg:"render_cutoff"`
		CoalesceThreshold int    `gcfg:"coalesce_threshold"`
		RenderAuto        int    `gcfg:"render_auto"`
		LastLogDir        string `gcfg:"last_log_dir"`
	}
}

/*
   #########################################################################
   # GUI setup
   #########################################################################
*/

type ContextViewer struct {
	// GUI
	master      *gtk.Window
	status      *gtk.Statusbar
	configFile  string
	config      Config
	lastLogDir  string
	windowReady bool

	// data
	c         *sqlite3.Conn
	data      []Event
	bookmarks *gtk.ListStore
	summary   []int
	threads   []string
	logStart  float64
	logEnd    float64

	// rendering
	eventIdxOffset float64
	renderStart    float64
	renderLen      float64
	renderCutoff   float64
	renderScale    float64
	coalesceThreshold float64
}

func (self *ContextViewer) Init(master *gtk.Window, databaseFile *string) {
	usr, _ := user.Current()

	self.master = master
	self.windowReady = false
	/*
	   self.original_texts = {}
	   self.tooltips = {}
	*/
	self.config.Gui.LastLogDir = usr.HomeDir

	/*
	   try:
	       os.makedirs(os.path.expanduser(os.path.join("~", ".config")))
	   except OSError:
	       pass
	*/
	//        self.configFile = os.path.expanduser(os.path.join("~", ".config", "context.cfg"))
	self.configFile = usr.HomeDir + "/.config/context.cfg"
	self.renderScale = 50.0
	self.renderLen = 10.0
	/*
	   self.threads = []
	   self.render_start = DoubleVar(master, 0)
	   self.render_len = IntVar(master, 10)
	   self.render_cutoff = IntVar(master, 1)
	   self.coalesce_threshold = IntVar(master, 1)
	   self.render_auto = IntVar(master, 1)
	   self.scale = IntVar(master, 1000)
	*/

	self.LoadSettings()

	master.Connect("destroy", func() {
		self.SaveSettingsAndQuit()
	})
	/*
	   self.render_start.trace_variable("w", lambda *x: conditional(self.render_auto, self.update))
	   self.render_len.trace_variable("w", lambda *x: conditional(self.render_auto, self.update))
	   self.render_cutoff.trace_variable("w", lambda *x: conditional(self.render_auto, self.render))
	   self.coalesce_threshold.trace_variable("w", lambda *x: conditional(self.render_auto, self.update))
	   self.scale.trace_variable("w", lambda *x: conditional(self.render_auto, self.render))

	   self.img_start = PhotoImage(data=data.start)
	   self.img_prev = PhotoImage(data=data.prev)
	   self.img_next = PhotoImage(data=data.next)
	   self.img_end = PhotoImage(data=data.end)
	   self.img_logo = PhotoImage(data=data.context_name)
	*/

	// Create a new label widget to show in the window.
	grid, err := gtk.GridNew()
	grid.SetOrientation(gtk.ORIENTATION_VERTICAL)
	master.Add(grid)

	menuBar := self.__menu()
	controls := self.__controlBox()
	bookmarks := self.__bookmarks()
	canvas := self.__canvas()
	scrubber := self.__scrubber()

	status, err := gtk.StatusbarNew()
	if err != nil {
		log.Fatal("Unable to create label:", err)
	}

	grid.Attach(menuBar, 0, 0, 2, 1)
	grid.Attach(controls, 0, 1, 2, 1)
	grid.Attach(bookmarks, 0, 2, 1, 1)
	grid.Attach(canvas, 1, 2, 1, 1)
	grid.Attach(scrubber, 0, 3, 2, 1)
	grid.Attach(status, 0, 4, 2, 1)

	self.status = status
	self.SetStatus("test")

	self.windowReady = true

	if databaseFile != nil {
		self.LoadFile(*databaseFile)
	}
}

func (self *ContextViewer) __menu() *gtk.MenuBar {
	menuBar, err := gtk.MenuBarNew()
	if err != nil {
		log.Fatal("Unable to create label:", err)
	}

	menu, err := gtk.MenuNew()
	menuBar.Add(menu)

	item, err := gtk.MenuItemNewWithLabel("Item!")
	//item.SetText("Menu!")
	menu.Add(item)

	return menuBar
}

/*
   def __menu(self, master):
       menubar = Menu(master)

       def file_menu():
           menu = Menu(menubar, tearoff=0)
           menu.add_command(label="Open ctxt / cbin", command=self.open_file)
           # menu.add_command(label="Append ctxt", command=self.append_file)
           menu.add_separator()
           menu.add_command(label="Exit", command=self.save_settings_and_quit)
           return menu
       menubar.add_cascade(label="File", menu=file_menu())

       def view_menu():
           menu = Menu(menubar, tearoff=0)
           menu.add_checkbutton(label="Auto-render", variable=self.render_auto)
           # menu.add_command(label="Filter threads", command=None)
           return menu
       menubar.add_cascade(label="View", menu=view_menu())

       def analyse_menu():
           # def timechart():
           #    _TimeChart(master, self.output.get("0.0", END))
           menu = Menu(menubar, tearoff=0)
           # menu.add_command(label="Time Chart", command=timechart)
           return menu
       # menubar.add_cascade(label="Analyse", menu=analyse_menu())

       def help_menu():
           def show_about():
               t = Toplevel(master)
               t.title("About")
               t.transient(master)
               t.resizable(False, False)
               Label(t, image=self.img_logo).grid(column=0, row=0, sticky=(E, W))
               Label(t, text="Context %s" % VERSION, anchor=CENTER).grid(column=0, row=1, sticky=(E, W))
               Label(t, text="(c) 2011-2014 Shish", anchor=CENTER).grid(column=0, row=2, sticky=(E, W))
               Button(t, text="Close", command=t.destroy).grid(column=0, row=3, sticky=(E,))
               win_center(t)

           def show_docs():
               t = Toplevel(master)
               t.title("Context Documentation")
               t.transient(master)
               scroll = Scrollbar(t, orient=VERTICAL)
               tx = Text(
                   t,
                   wrap=WORD,
                   yscrollcommand=scroll.set,
               )
               scroll['command'] = tx.yview
               scroll.pack(side=RIGHT, fill=Y, expand=1)
               tx.pack(fill=BOTH, expand=1)
               tx.insert("0.0", b64decode(data.README).replace("\r", ""))
               tx.configure(state="disabled")
               tx.focus_set()
               win_center(t)

           def show_license():
               t = Toplevel(master)
               t.title("Context Licenses")
               t.transient(master)
               scroll = Scrollbar(t, orient=VERTICAL)
               tx = Text(
                   t,
                   wrap=WORD,
                   yscrollcommand=scroll.set,
               )
               scroll['command'] = tx.yview
               scroll.pack(side=RIGHT, fill=Y, expand=1)
               tx.pack(fill=BOTH, expand=1)
               tx.insert("0.0", b64decode(data.LICENSE).replace("\r", ""))
               tx.configure(state="disabled")
               tx.focus_set()
               win_center(t)

           menu = Menu(menubar, tearoff=0)
           menu.add_command(label="About", command=show_about)
           menu.add_command(label="Documentation", command=show_docs)
           menu.add_command(label="License", command=show_license)
           return menu
       menubar.add_cascade(label="Help", menu=help_menu())

       return menubar
*/

func (self *ContextViewer) __controlBox() *gtk.Grid {
	grid, err := gtk.GridNew()
	if err != nil {
		log.Fatal("Unable to create grid:", err)
	}
	grid.SetOrientation(gtk.ORIENTATION_HORIZONTAL)

	l, _ := gtk.LabelNew("Start")
	grid.Add(l)

	start, _ := gtk.SpinButtonNewWithRange(0, 9999999999999, 0.1)
	grid.Add(start)

	l, _ = gtk.LabelNew("Seconds")
	grid.Add(l)

	sec, _ := gtk.SpinButtonNewWithRange(MIN_SEC, MAX_SEC, 1.0)
	grid.Add(sec)

	l, _ = gtk.LabelNew("Pixels Per Second")
	grid.Add(l)

	pps, _ := gtk.SpinButtonNewWithRange(MIN_PPS, MAX_PPS, 10.0)
	grid.Add(pps)

	label, _ := gtk.ButtonNewWithLabel("Render!")
	grid.Add(label)

	return grid
}

/*
       _la("  Start ")
       _sp(0, int(time.time()), 10, self.render_start, 15)
       _la("  Seconds ")
       _sp(MIN_SEC, MAX_SEC, 1, self.render_len, 3)
       _la("  Pixels per second ")
       _sp(MIN_PPS, MAX_PPS, 100, self.scale, 5)

       _la("  Cutoff (ms) ")
       _sp(0, 1000, 1, self.render_cutoff, 3)
       _la("  Coalesce (ms) ")
       _sp(0, 1000, 1, self.coalesce_threshold, 3)
       Button(f, text="Render", command=self.update).pack(side=LEFT, fill=Y)  # padding=0

       f.pack()
       return f
*/

func (self *ContextViewer) __bookmarks() *gtk.Grid {
	grid, _ := gtk.GridNew()

	self.bookmarks, _ = gtk.ListStoreNew(glib.TYPE_DOUBLE, glib.TYPE_STRING)

	bookmarkScroller, _ := gtk.ScrolledWindowNew(nil, nil)
	bookmarkScroller.SetSizeRequest(250, 200)
	bookmarkView, _ := gtk.TreeViewNewWithModel(self.bookmarks)
	bookmarkView.SetVExpand(true)
	bookmarkScroller.Add(bookmarkView)
	grid.Attach(bookmarkScroller, 0, 0, 5, 1)

	renderer, _ := gtk.CellRendererTextNew()
	column, _ := gtk.TreeViewColumnNewWithAttribute("Bookmarks", renderer, "text", 1)
	bookmarkView.AppendColumn(column)

	l, _ := gtk.ButtonNewWithLabel("<<")
	grid.Attach(l, 0, 1, 1, 1)

	l, _ = gtk.ButtonNewWithLabel("<")
	grid.Attach(l, 1, 1, 1, 1)

	//l, _ = gtk.ButtonNewWithLabel(" ")
	//grid.Attach(l, 2, 1, 1, 1)

	l, _ = gtk.ButtonNewWithLabel(">")
	grid.Attach(l, 3, 1, 1, 1)

	l, _ = gtk.ButtonNewWithLabel(">>")
	grid.Attach(l, 4, 1, 1, 1)

	return grid
}

/*
   def __bookmarks(self, master):
       self.bookmarks_list = li

       def _lbox_selected(*args):
           selected_idx = int(li.curselection()[0])
           self.render_start.set(self.bookmarks_values[selected_idx])
           self.canvas.xview_moveto(0)
           if not self.render_auto.get():
               self.update()
       li.bind('<Double-Button-1>', _lbox_selected)

       Button(buttons, image=self.img_start, command=self.start_event).pack(side="left")
       Button(buttons, image=self.img_prev, command=self.prev_event).pack(side="left")
       Button(buttons, image=self.img_end, command=self.end_event).pack(side="right")
       Button(buttons, image=self.img_next, command=self.next_event).pack(side="right")
*/

func (self *ContextViewer) __canvas() *gtk.Grid {
	grid, _ := gtk.GridNew()

	canvasScroller, _ := gtk.ScrolledWindowNew(nil, nil)
	canvasScroller.SetSizeRequest(250, 200)

	canvas, _ := gtk.DrawingAreaNew()
	canvas.SetSizeRequest(2000, 20)
	canvas.SetHExpand(true)
	canvas.SetVExpand(true)
	canvas.Connect("draw", func(widget *gtk.DrawingArea, cr *cairo.Context) {
		self.RenderBase(cr)
		self.RenderData(cr)
	})

	canvasScroller.Add(canvas)
	grid.Add(canvasScroller)

	return grid
}

/*
   canvas.bind("<4>", lambda e: self.scale_view(e, 1.0 * 1.1))
   canvas.bind("<5>", lambda e: self.scale_view(e, 1.0 / 1.1))

   # in windows, mouse wheel events always go to the root window o_O
   self.master.bind("<MouseWheel>", lambda e: self.scale_view(
       e, ((1.0 * 1.1) if e.delta > 0 else (1.0 / 1.1))
   ))

   # Drag based movement
   # def _sm(e):
   #    self.st = self.render_start.get()
   #    self.sx = e.x
   #    self.sy = e.y
   # def _cm(e):
   #    self.render_start.set(self.st + float(self.sx - e.x)/self.scale.get())
   #    self.render()
   # self.canvas.bind("<1>", _sm)
   # self.canvas.bind("<B1-Motion>", _cm)
*/

func (self *ContextViewer) __scrubber() *gtk.Grid {
	grid, _ := gtk.GridNew()

	canvas, _ := gtk.DrawingAreaNew()
	canvas.SetSizeRequest(200, SCRUBBER_HEIGHT)
	canvas.SetHExpand(true)
	canvas.Connect("draw", func(widget *gtk.DrawingArea, cr *cairo.Context) {
		self.RenderScrubber(cr)
	})
	grid.Add(canvas)

	return grid
}

/*
   def sc_goto(e):
       width_fraction = float(e.x) / sc.winfo_width()
       ev_s = self.get_earliest_bookmark_after(0)
       ev_e = self.get_latest_bookmark_before(sys.maxint)
       ev_l = ev_e - ev_s
       self.render_start.set(ev_s + ev_l * width_fraction - float(self.render_len.get()) / 2)
       if not self.render_auto.get():
           self.update()
       self.canvas.xview_moveto(0.5)
   sc.bind("<1>", sc_goto)

   def resize(event):
       if self.c:
           self.render_scrubber_activity()
           self.render_scrubber_arrow()
       # sc.coords(line, 0, 0, event.width, event.height)
   sc.bind("<Configure>", resize)
*/

func (self *ContextViewer) SetStatus(text string) {
	if text != "" {
		fmt.Println(text)
	}
	self.status.Pop(0) // RemoveAll?
	self.status.Push(0, text)
}

func (self *ContextViewer) ShowError(title, text string) {
	log.Printf("%s: %s\n", title, text)
	// TODO
}

/*
   #########################################################################
   # Open file
   #########################################################################
*/

func (self *ContextViewer) OpenFile() {
	var filename *string
	/*
	   filename = askopenfilename(
	       filetypes=[
	           ("All Supported Types", "*.ctxt *.cbin"),
	           ("Context Text", "*.ctxt"),
	           ("Context Binary", "*.cbin")
	       ],
	       initialdir=self._last_log_dir
	   )
	*/
	if filename != nil {
		self.LoadFile(*filename)
		//		if err != nil {
		//			self.SetStatus("Error loading file: %s" % str(e))
		//		}
	}
}

func (self *ContextViewer) LoadFile(givenFile string) {
	fmt.Printf("Loading file: %s\n", givenFile)

	self.config.Gui.LastLogDir = filepath.Dir(givenFile)
	/*
	   path, _ext = os.path.splitext(givenFile)
	*/
	path := strings.Split(givenFile, ".")[0]
	logFile := path + ".ctxt"
	databaseFile := path + ".cbin"

	logStat, err := os.Stat(logFile)
	if err != nil {
		self.ShowError("Error", fmt.Sprintf("Context dump file '%s' does not exist", givenFile))
		return
	}

	// if the user picked a log file, compile it (unless an
	// up-to-date version already exists)
	if givenFile == logFile {
		needsRecompile := false

		databaseStat, err := os.Stat(databaseFile)

		if err != nil {
			needsRecompile = true
			self.SetStatus("Compiled log not found, compiling")
		} else if logStat.ModTime().UnixNano() > databaseStat.ModTime().UnixNano() {
			needsRecompile = true
			self.SetStatus("Compiled log is out of date, recompiling")
		} else if !VersionCheck(databaseFile) {
			needsRecompile = true
			self.SetStatus("Compiled log is from an old version of context, recompiling")
		}

		if needsRecompile {
			compiler := exec.Command("context-compiler", logFile)
			pipe, _ := compiler.StdoutPipe()
			reader := bufio.NewScanner(pipe)
			compiler.Start()

			for reader.Scan() {
				line := reader.Text()
				if line != "" {
					self.SetStatus(strings.Trim(line, "\n\r"))
				} else {
					break
				}
			}
		}
	}

	// load the data
	self.master.SetTitle(NAME + ": " + databaseFile)
	if self.c != nil {
		self.c.Close()
		self.c = nil
	}
	self.c, err = sqlite3.Open(databaseFile)

	self.data = []Event{} // don't load the bulk of the data yet
	self.LoadBookmarks(self.c)
	self.LoadSummary(self.c)
	self.LoadThreads(self.c)

	// render grid + scrubber
	self.Render()

	self.eventIdxOffset = self.GetEarliestBookmarkAfter(0)

	//	self.renderStart.set(self.eventIdxOffset)
	self.renderStart = self.eventIdxOffset
	self.Update() // the above should do this
}

func (self *ContextViewer) LoadBookmarks(conn *sqlite3.Conn) {
	self.bookmarks.Clear()

	sql := "SELECT start_time, start_text, end_text FROM events WHERE start_type = 'BMARK' ORDER BY start_time"
	for query, err := conn.Query(sql); err == nil; err = query.Next() {
		var startTime float64
		var startText, endText string
		query.Scan(&startTime, &startText, &endText)

		itemPtr := self.bookmarks.Append()
		text := fmt.Sprintf("%19.19s: %s", time.Unix(int64(startTime), 0), startText)
		self.bookmarks.Set(itemPtr, []int{0, 1}, []interface{}{startTime, text})
	}
}

func (self *ContextViewer) LoadThreads(conn *sqlite3.Conn) {
	self.threads = make([]string, 0, 10)

	sql := "SELECT node, process, thread FROM threads ORDER BY id"
	for query, err := conn.Query(sql); err == nil; err = query.Next() {
		var node, process, thread string
		query.Scan(&node, &process, &thread)
		self.threads = append(self.threads, fmt.Sprintf("%s-%s-%s", node, process, thread))
	}
}

func (self *ContextViewer) LoadSummary(conn *sqlite3.Conn) {
	self.summary = make([]int, 0, 1000)

	sql := "SELECT events FROM summary ORDER BY id"
	for query, err := conn.Query(sql); err == nil; err = query.Next() {
		var val int
		query.Scan(&val)
		self.summary = append(self.summary, val)
	}

	// TODO: bake this into the .cbin
	sql = "SELECT min(start_time), max(end_time) FROM events"
	for query, err := conn.Query(sql); err == nil; err = query.Next() {
		query.Scan(&self.logStart, &self.logEnd)
		//fmt.Printf("Range: %d:%d", self.logStart, self.logEnd)
	}
}

/*
   #########################################################################
   # Settings
   #########################################################################
*/

func (self *ContextViewer) LoadSettings() {
	err := gcfg.ReadFileInto(&self.config, self.configFile)
	if err != nil {
		fmt.Printf("Error loading settings from %s:\n  %s\n", self.configFile, err)
	}
}

func (self *ContextViewer) SaveSettings() {
	/*
	   try:
	       cp = ConfigParser.SafeConfigParser()
	       cp.add_section("gui")
	       cp.set("gui", "render_len", str(self.render_len.get()))
	       cp.set("gui", "scale", str(self.scale.get()))
	       cp.set("gui", "render_cutoff", str(self.render_cutoff.get()))
	       cp.set("gui", "coalesce_threshold", str(self.coalesce_threshold.get()))
	       cp.set("gui", "render_auto", str(self.render_auto.get()))
	       cp.set("gui", "last_log_dir", self._last_log_dir)
	       cp.write(file(self.config_file, "w"))
	   except Exception as e:
	       print("Error writing settings to %s:\n  %s" % (self.config_file, e))
	*/
}

func (self *ContextViewer) SaveSettingsAndQuit() {
	self.SaveSettings()
	gtk.MainQuit()
}

/*
   #########################################################################
   # Navigation
   #########################################################################
*/

func (self *ContextViewer) GetEarliestBookmarkAfter(startHint float64) float64 {
	var startTime float64
	sql := "SELECT min(start_time) FROM events WHERE start_time > ? AND start_type = 'BMARK'"
	for query, err := self.c.Query(sql, startHint); err == nil; err = query.Next() {
		query.Scan(&startTime)
	}
	return startTime
}

func (self *ContextViewer) GetLatestBookmarkBefore(endHint float64) float64 {
	var endTime float64
	sql := "SELECT max(start_time) FROM events WHERE start_time < ? AND start_type = 'BMARK'"
	for query, err := self.c.Query(sql, endHint); err == nil; err = query.Next() {
		query.Scan(&endTime)
	}
	return endTime
}

func (self *ContextViewer) EndEvent() {
	self.renderStart = self.logEnd - self.renderLen
	// self.canvas.xview_moveto(0)
}

func (self *ContextViewer) NextEvent() {
	ts := self.GetEarliestBookmarkAfter(self.renderStart)
	if ts != 0.0 {
		// self.render_start.set(ts)
	}
	// self.canvas.xview_moveto(0)
}

func (self *ContextViewer) PrevEvent() {
	ts := self.GetLatestBookmarkBefore(self.renderStart)
	if ts != 0.0 {
		// self.render_start.set(ts)
	}
	// self.canvas.xview_moveto(0)
}

func (self *ContextViewer) StartEvent() {
	self.renderStart = self.logStart
	// self.canvas.xview_moveto(0)
}

/*
   #########################################################################
   # Rendering
   #########################################################################
*/
/*
   def scale_view(self, e=None, n=1.0):
       # get the old pos
       if e:
           _xv = self.canvas.xview()
           left_edge = _xv[0]
           width = _xv[1] - _xv[0]
           width_fraction = float(e.x) / self.canvas.winfo_width()
           x_pos = left_edge + width * width_fraction
       # scale
       if n != 1:
           self.soft_scale *= n
           self.canvas.scale("event", 0, 0, n, 1)
           self.canvas.scale("lock", 0, 0, n, 1)
           for t in self.canvas.find_withtag("time_label"):
               val = self.canvas.itemcget(t, 'text')[2:]
               self.canvas.itemconfigure(t, text=" +%.4f" % (float(val) / n))
           for t in self.canvas.find_withtag("event_tip"):
               self.canvas.itemconfigure(t, width=float(self.canvas.itemcget(t, 'width')) * n)  # this seems slow? sure something similar was faster...
           for t in self.canvas.find_withtag("event_label"):
               self.canvas.itemconfigure(t, width=float(self.canvas.itemcget(t, 'width')) * n)  # this seems slow? sure something similar was faster...
               w = int(self.canvas.itemcget(t, 'width'))
               tx = self.truncate_text(" " + self.original_texts[t], w)
               self.canvas.itemconfigure(t, text=tx)  # this seems slow? sure something similar was faster...
           self.canvas.delete("grid")
           self.render_base()
           self.canvas.configure(scrollregion=shrink(self.canvas.bbox("grid"), 2))
       # scroll the canvas so that the mouse still points to the same place
       if e:
           _xv = self.canvas.xview()
           new_width = _xv[1] - _xv[0]
           self.canvas.xview_moveto(x_pos - new_width * width_fraction)

   def truncate_text(self, text, w):
       return text.split("\n")[0][:w / self.char_w]
*/

func (self *ContextViewer) Update() {
	self.UpdateEvents()
	self.Render()
}

// Data settings changed, get new data and re-render
func (self *ContextViewer) UpdateEvents() {
	if !self.windowReady {
		// update() is called a couple of times during init()
		return
	}

	defer self.SetStatus("")
	s := self.renderStart
	e := self.renderStart + self.renderLen
	threshold := float64(self.coalesceThreshold) / 1000.0
	self.data = []Event{} // free memory
	threadLevelEnds := make([][]Event, len(self.threads))

	/*
		n = 0
	   	func progress() {
	   		n++
	   		self.SetStatus("Loading... (%dk opcodes)" % (self.n * 10))
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
	for query, err := self.c.Query(sql, s-self.eventIdxOffset, e-self.eventIdxOffset, self.config.Gui.RenderCutoff); err == nil; err = query.Next() {
		var event Event
		event.NewEvent(query)
		thread_idx := event.thread_id // TODO: index into currently-active threads, not all threads

		if event.start_type == "START" {
			var prevEventAtLevel *Event
			endIdx := len(threadLevelEnds[thread_idx]) - 1
			for len(threadLevelEnds[thread_idx]) > 0 && threadLevelEnds[thread_idx][endIdx].end_time <= event.start_time {
				threadLevelEnds[thread_idx], prevEventAtLevel = threadLevelEnds[thread_idx][:endIdx], &threadLevelEnds[thread_idx][endIdx]
			}
			event.depth = len(threadLevelEnds[thread_idx])

			self.data = append(self.data, event)
			if false {
				fmt.Printf("%s %s\n", prevEventAtLevel, threshold)
			}

			/*
			if threshold > 0.0 &&
				prevEventAtLevel != nil &&
				prevEventAtLevel.CanMerge(event, threshold) {
				prevEventAtLevel.Merge(event)
				threadLevelEnds[thread_idx] = append(threadLevelEnds[thread_idx], *prevEventAtLevel)
			} else {
				threadLevelEnds[thread_idx] = append(threadLevelEnds[thread_idx], event)
				self.data = append(self.data, event)
			}
			*/
		} else {
			self.data = append(self.data, event)
		}
	}
}

// Render settings changed, re-render with existing data
func (self *ContextViewer) Render() {
	if !self.windowReady {
		return
	}
	self.RenderClear()
	//self.RenderScrubber()
	//self.RenderBase()
	//self.RenderData()
}

// clear the canvas and any cached variables
func (self *ContextViewer) RenderClear() {
	/*
	   self.canvas.delete(ALL)
	   self.original_texts = {}
	   self.tooltips = {}
	   self.canvas.configure(scrollregion=(
	       0, 0,
	       self.render_len.get() * self.scale.get(),
	       len(self.threads) * (MAX_DEPTH * BLOCK_HEIGHT) + HEADER_HEIGHT
	   ))
	   if self.char_w == -1:
	       t = self.canvas.create_text(0, 0, font="TkFixedFont", text="_", anchor=NW)
	       bb = self.canvas.bbox(t)
	       # [2]-[0]=10, but trying by hand, 8px looks better on win7
	       # 7px looks right on linux, not sure what [2]-[0] is there,
	       # hopefully 9px, so "-2" always helps?
	       self.char_w = bb[2] - bb[0] - 2
	       self.canvas.delete(t)
	*/
}

func (self *ContextViewer) RenderScrubber(cr *cairo.Context) {
	cr.SetSourceRGB(1, 1, 1)
	cr.Paint()

	activityPeak := 0
	for _, el := range self.summary {
		if el > activityPeak {
			activityPeak = el
		}
	}

	width := 500.0 // TODO: get from canvas / widget
	length := float64(len(self.summary))
	for n, el := range self.summary {
		frac := float64(el) / float64(activityPeak)
		cr.SetSourceRGB(frac, 1.0-frac, 0.0)
		cr.Rectangle(float64(n)/length*width, 0, width/length, SCRUBBER_HEIGHT)
		cr.Fill()
	}

	cr.SetSourceRGB(0, 0, 0)

	// events start / end / length
	ev_s := self.logStart
	ev_e := self.logEnd
	ev_l := ev_e - ev_s

	if ev_l == 0 { // only one event in the log o_O?
		return
	}

	// view start / end / length
	vi_s := self.renderStart
	vi_e := self.renderStart + self.renderLen
	//vi_l := vi_e - vi_s

	// scrubber width
	sc_w := width

	// arrow
	start_rel := vi_s - ev_s
	start := (start_rel / ev_l) * sc_w

	end_rel := vi_e - ev_s
	end := (end_rel / ev_l) * sc_w

	// left edge
	cr.MoveTo(start, 1)
	cr.LineTo(start, SCRUBBER_HEIGHT)
	cr.Stroke()

	cr.MoveTo(start, SCRUBBER_HEIGHT/2)
	cr.LineTo(start+5, 15)
	cr.Stroke()

	cr.MoveTo(start, SCRUBBER_HEIGHT/2)
	cr.LineTo(start+5, 5)
	cr.Stroke()

	// right edge
	cr.MoveTo(end, 1)
	cr.LineTo(end, SCRUBBER_HEIGHT)
	cr.Stroke()

	cr.MoveTo(end, SCRUBBER_HEIGHT/2)
	cr.LineTo(end-5, 15)
	cr.Stroke()

	cr.MoveTo(end, SCRUBBER_HEIGHT/2)
	cr.LineTo(end-5, 5)
	cr.Stroke()

	// join
	cr.MoveTo(start, SCRUBBER_HEIGHT/2)
	cr.LineTo(end, SCRUBBER_HEIGHT/2)
	cr.Stroke()
}

// Render grid lines and markers
func (self *ContextViewer) RenderBase(cr *cairo.Context) {
	cr.SetSourceRGB(1, 1, 1)
	cr.Paint()

	_rl := self.renderLen
	_sc := self.renderScale

	rs_px := _rl * _sc
	rl_px := _rl * _sc

	//pangocairo_context := pangocairo.CairoContext(cr)
	//layout := pangocairo_context.create_layout()
	//layout.SetAlignment(pango.ALIGN_LEFT)
	//layout.SetWrap(pango.WRAP_WORD_CHAR)
	//layout.SetFontDescription(pango.FontDescription("Arial 10"))

	cr.SetSourceRGB(0.8, 0.8, 0.8) // #CCC
	cr.SetLineWidth(1.0)
	for n := rs_px; n < rs_px+rl_px; n += 100 {
		cr.MoveTo(n-rs_px, 0)
		cr.LineTo(n-rs_px, float64(HEADER_HEIGHT+len(self.threads)*MAX_DEPTH*BLOCK_HEIGHT))
		cr.Stroke()

		//label := fmt.Sprintf(" +%.4f", float64(n) / _sc - _rl)
		//self.canvas.create_text(n - rs_px, 5, text=label, anchor=NW, tags="time_label grid")
		//layout.SetText(label)
		//layout.SetWidth(r[1][2] * pango.SCALE)
		//cr.translate(r[1][0]+2, r[1][1]+1)
		//pangocairo_context.UpdateLayout(layout)
		//pangocairo_context.ShowLayout(layout)
		//cr.Translate(-r[1][0]-2, -r[1][1]-1)
	}

	cr.SetSourceRGB(0.75, 0.75, 0.75) // #CCC
	cr.SetLineWidth(1.0)
	for n, _ := range self.threads {
		cr.MoveTo(0, float64(HEADER_HEIGHT+MAX_DEPTH*BLOCK_HEIGHT*n))
		cr.LineTo(rl_px, float64(HEADER_HEIGHT+MAX_DEPTH*BLOCK_HEIGHT*n))
		cr.Stroke()

		//self.canvas.create_text(0, HEADER_HEIGHT + MAX_DEPTH * BLOCK_HEIGHT * (n + 1) - 5, text=" " + self.threads[n], anchor=SW, tags="grid")
	}
}

// add the event rectangles
func (self *ContextViewer) RenderData(cr *cairo.Context) {
	if !self.windowReady {
		// update() is called a couple of times during init()
		return
	}

	_rs := self.renderStart
	_rc := self.renderCutoff
	_sc := 50.0 // self.scale

	eventCount := len(self.data) - 1
	shown := 0
	for n, event := range self.data {
		if n%1000 == 0 || n == eventCount {
			self.SetStatus(fmt.Sprintf("Rendered %d events (%.0f%%)", n, float64(n)*100.0/float64(eventCount)))
			//self.master.update()
		}
		thread_idx := event.thread_id

		if event.start_type == "START" {
			if (event.end_time-event.start_time)*1000 < _rc {
				continue
			}
			if event.depth >= MAX_DEPTH {
				continue
			}
			shown += 1
			//			if shown == 500 && VERSION.endswith("-demo") {
			//				self.ShowError("Demo Limit", "The evaluation build is limited to showing 500 events at a time, so rendering has stopped")
			//				break
			//			}
			self.ShowEvent(
				cr,
				&event, _rs, _sc,
				thread_idx,
			)
		} else if event.start_type == "BMARK" {
			// note that when loading data, we currently filter for # "start_type=START" for a massive indexed speed boost
			// so there are no bookmarks. We may want to load bookmarks
			// into a separate array?
			//pass  // render bookmark
		} else if event.start_type == "LOCKW" || event.start_type == "LOCKA" {
			self.ShowLock(
				cr,
				&event, _rs, _sc,
				thread_idx,
			)
		}
	}

	self.SetStatus("")
}

func (self *ContextViewer) ShowEvent(cr *cairo.Context, event *Event, offset_time, scale_factor float64, thread int) {
	ok := event.end_type == "ENDOK"

	start_px := (event.start_time - offset_time) * scale_factor
	length_px := event.Length() * scale_factor

	//	tip := fmt.Sprintf("%dms @%dms: %s\n%s",
	//	   (event.end_time - event.start_time) * 1000,
	//	   (event.start_time - offset_time) * 1000,
	//	   event.start_location, event.Text())

	//outl := "#484" if ok else "#844"
	if ok {
		cr.SetSourceRGB(0.8, 1.0, 0.8)
	} else {
		cr.SetSourceRGB(0.8, 0.8, 1.0)
	}
	cr.Rectangle(
		start_px, float64(HEADER_HEIGHT+thread*MAX_DEPTH*BLOCK_HEIGHT+event.depth*BLOCK_HEIGHT),
		length_px, BLOCK_HEIGHT,
	)
	cr.Fill()

	if ok {
		cr.SetSourceRGB(0.3, 0.5, 0.3)
	} else {
		cr.SetSourceRGB(0.5, 0.3, 0.3)
	}
	cr.Rectangle(
		start_px, float64(HEADER_HEIGHT+thread*MAX_DEPTH*BLOCK_HEIGHT+event.depth*BLOCK_HEIGHT),
		length_px, BLOCK_HEIGHT,
	)
	cr.Stroke()
	/*
		t = self.canvas.create_text(
		   start_px, HEADER_HEIGHT + thread * MAX_DEPTH * BLOCK_HEIGHT + event.depth * BLOCK_HEIGHT + 3,
		   text=self.truncate_text(" " + event.text, length_px), tags="event event_label", anchor=NW, width=length_px,
		   font="TkFixedFont",
		   state="disabled",
		)
		self.canvas.tag_raise(r)
		self.canvas.tag_raise(t)

		self.canvas.tag_bind(r, "<1>", lambda e: self._focus(r))

		self.original_texts[t] = event.text
		self.tooltips[r] = tip

		self.canvas.tag_bind(r, "<Enter>", lambda e: self._ttip_show(r))
		self.canvas.tag_bind(r, "<Leave>", lambda e: self._ttip_hide())
	*/
}

func (self *ContextViewer) ShowLock(cr *cairo.Context, event *Event, offset_time, scale_factor float64, thread int) {
	start_px := (event.start_time - offset_time) * scale_factor
	length_px := event.Length() * scale_factor

	// fill = "#FDD" if event.start_type == "LOCKW" else "#DDF"
	if event.start_type == "LOCKW" {
		cr.SetSourceRGB(1.0, 0.85, 0.85)
	} else {
		cr.SetSourceRGB(0.85, 0.85, 1.0)
	}
	cr.Rectangle(
	   start_px, float64(HEADER_HEIGHT + thread * MAX_DEPTH * BLOCK_HEIGHT),
	   length_px, float64(MAX_DEPTH * BLOCK_HEIGHT),
	)
	cr.Fill()

	/*
	t = self.canvas.create_text(
	   start_px + length_px, HEADER_HEIGHT + (thread + 1) * MAX_DEPTH * BLOCK_HEIGHT,
	   text=self.truncate_text(event.text, length_px),
	   tags="lock lock_label", anchor=SE, width=length_px,
	   font="TkFixedFont",
	   state="disabled",
	   fill="#888",
	)
	*/
}

/*
   def _focus(self, r):
       # scale the canvas so that the (selected item width + padding == screen width)
       view_w = self.canvas.winfo_width()
       rect_w = max(self.canvas.bbox(r)[2] - self.canvas.bbox(r)[0] + HEADER_HEIGHT, 10)
       self.scale_view(n=float(view_w) / rect_w)

       # move the view so that the selected (item x1 = left edge of screen + padding)
       canvas_w = self.canvas.bbox("grid")[2]
       rect_x = self.canvas.bbox(r)[0] - 5
       self.canvas.xview_moveto(float(rect_x) / canvas_w)

   def _ttip_show(self, r):
       tip = self.tooltips[r]

       x0, y0, x1, y1 = self.canvas.bbox(r)

       if x0 < 0:
           x1 = x1 - x0
           x0 = x0 - x0

       t2 = self.canvas.create_text(
           x0 + 4, y0 + BLOCK_HEIGHT + 4,
           text=tip.strip(), width=400, tags="tooltip", anchor=NW,
           justify="left", state="disabled",
       )

       x0, y0, x1, y1 = self.canvas.bbox(t2)

       r2 = self.canvas.create_rectangle(
           x0 - 2, y0 - 1, x1 + 2, y1 + 2,
           state="disabled", fill="#FFA", outline="#AA8", tags="tooltip"
       )

       self.canvas.tag_raise(t2)

   def _ttip_hide(self):
       self.canvas.delete("tooltip")
*/

func main() {
	var filename *string

	/*
	   parser = OptionParser()
	   parser.add_option("-g", "--geometry", dest="geometry", default="1000x600",
	                     help="location and size of window", metavar="GM")
	   parser.add_option("-d", "--depth", dest="depth", default=7,
	                     type=int, help="how many rows to show in each stack", metavar="DEPTH")
	   (options, args) = parser.parse_args(argv)

	   # lol constants
	   global MAX_DEPTH
	   MAX_DEPTH = options.depth
	*/

	if len(os.Args) > 1 {
		filename = &os.Args[1]
	}

	gtk.Init(nil)
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		log.Fatal("Unable to create window:", err)
	}

	win.SetTitle(NAME)
	//	set_icon(root, "images/tools-icon")

	cv := ContextViewer{}
	cv.Init(win, filename)

	// Set the default window size.
	// TODO: options.geometry
	win.SetDefaultSize(1000, 600)

	// Recursively show all widgets contained in this window.
	win.ShowAll()

	// Begin executing the GTK main loop.  This blocks until
	// gtk.MainQuit() is run.
	gtk.Main()
}
