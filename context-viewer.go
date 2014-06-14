package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	// gtk
	"github.com/conformal/gotk3/cairo"
	"github.com/conformal/gotk3/gdk"
	"github.com/conformal/gotk3/glib"
	"github.com/conformal/gotk3/gtk"
	//"github.com/conformal/gotk3/pango"
	"./viewer"
	"./common"
)

const (
	NAME            = "Context"
	BLOCK_HEIGHT    = 20
	HEADER_HEIGHT   = 20
	SCRUBBER_HEIGHT = 20
	MIN_PPS         = 1
	MAX_PPS         = 5000
	MIN_SEC         = 1
	MAX_SEC         = 600
)

/**********************************************************************
* Structs
**********************************************************************/

type Geometry struct {
	w int
	h int
}

type ContextViewer struct {
	name          string
	// GUI
	master        *gtk.Window
	canvas        *gtk.DrawingArea
	scrubber      *gtk.DrawingArea
	status        *gtk.Statusbar
	bookmarkPanel *gtk.Grid
	configFile    string
	config        viewer.Config

	// data
	data viewer.Data

	controls struct {
		active bool
		start  *gtk.SpinButton
		length *gtk.SpinButton
		scale  *gtk.SpinButton
	}
}

/**********************************************************************
* GUI Setup
**********************************************************************/

func (self *ContextViewer) Init(databaseFile *string, geometry Geometry) {
	master, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		log.Fatal("Unable to create window:", err)
	}

	self.name = NAME
	if common.DEMO {
		self.name += ": Non-commercial / Evaluation Version"
	}
	master.SetTitle(self.name)
	master.SetDefaultSize(geometry.w, geometry.h)
	//master.SetDefaultIcon(nil)  // TODO: set icon

	self.master = master

	usr, _ := user.Current()
	self.configFile = usr.HomeDir + "/.config/context2.cfg"
	err = self.config.Load(self.configFile)
	if err != nil {
		log.Printf("Error loading config file %s: %s\n", self.configFile, err)
	}

	master.Connect("destroy", self.Exit)

	menuBar := self.__menu()
	controls := self.__controlBox()
	bookmarks := self.__bookmarks()
	canvas := self.__canvas()
	scrubber := self.__scrubber()

	status, err := gtk.StatusbarNew()
	if err != nil {
		log.Fatal("Unable to create label:", err)
	}

	grid, err := gtk.GridNew()
	grid.Attach(menuBar, 0, 0, 2, 1)
	grid.Attach(controls, 0, 1, 2, 1)
	grid.Attach(bookmarks, 0, 2, 1, 1)
	grid.Attach(canvas, 1, 2, 1, 1)
	grid.Attach(scrubber, 0, 3, 2, 1)
	grid.Attach(status, 0, 4, 2, 1)
	master.Add(grid)

	self.bookmarkPanel = bookmarks
	self.status = status

	self.controls.active = true
	master.ShowAll()

	if !self.config.Gui.BookmarkPanel {
		self.bookmarkPanel.Hide()
	}

	if databaseFile != nil {
		self.LoadFile(*databaseFile)
	}
}

func (self *ContextViewer) Exit() {
	err := self.config.Save(self.configFile)
	if err != nil {
		log.Printf("Error saving settings to %s: %s\n", self.configFile, err)
	}
	gtk.MainQuit()
}

func (self *ContextViewer) __menu() *gtk.MenuBar {
	menuBar, _ := gtk.MenuBarNew()

	fileButton, _ := gtk.MenuItemNewWithLabel("File")
	fileButton.SetSubmenu(func() *gtk.Menu {
		fileMenu, _ := gtk.MenuNew()

		openButton, _ := gtk.MenuItemNewWithLabel("Open .ctxt / .cbin")
		openButton.Connect("activate", func() {
			var filename *string

			// TODO: pick a file
			//dialog := gtk.FileChooserNew()//title="Select a File", action=gtk.FILE_CHOOSER_ACTION_OPEN,
			//buttons=(gtk.STOCK_CANCEL, gtk.RESPONSE_CANCEL, gtk.STOCK_OPEN, gtk.RESPONSE_OK))

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
				self.config.Gui.LastLogDir = filepath.Dir(*filename)
				self.LoadFile(*filename)
			}
		})
		fileMenu.Append(openButton)

		sep, _ := gtk.SeparatorMenuItemNew()
		fileMenu.Append(sep)

		quitButton, _ := gtk.MenuItemNewWithLabel("Quit")
		quitButton.Connect("activate", self.Exit)
		fileMenu.Append(quitButton)

		return fileMenu
	}())
	menuBar.Append(fileButton)

	viewButton, _ := gtk.MenuItemNewWithLabel("View")
	viewButton.SetSubmenu(func() *gtk.Menu {
		viewMenu, _ := gtk.MenuNew()

		showBookmarkPanelButton, _ := gtk.CheckMenuItemNewWithLabel("Show Bookmark Panel")
		showBookmarkPanelButton.SetActive(self.config.Gui.BookmarkPanel)
		showBookmarkPanelButton.Connect("activate", func() {
			self.config.Gui.BookmarkPanel = showBookmarkPanelButton.GetActive()
			if self.config.Gui.BookmarkPanel {
				self.bookmarkPanel.Show()
			} else {
				self.bookmarkPanel.Hide()
			}
		})
		viewMenu.Append(showBookmarkPanelButton)

		showBookmarksButton, _ := gtk.CheckMenuItemNewWithLabel("Render Bookmarks")
		showBookmarksButton.SetActive(self.config.Render.Bookmarks)
		showBookmarksButton.Connect("activate", func() {
			self.config.Render.Bookmarks = showBookmarksButton.GetActive()
			self.canvas.QueueDraw()
		})
		viewMenu.Append(showBookmarksButton)

		sep, _ := gtk.SeparatorMenuItemNew()
		viewMenu.Append(sep)

		formatButton, _ := gtk.MenuItemNewWithLabel("Bookmark Time Format")
		formatButton.SetSubmenu(func() *gtk.Menu {
			formatMenu, _ := gtk.MenuNew()

			grp := &glib.SList{}

			bookmarksDateButton, _ := gtk.RadioMenuItemNewWithLabel(grp, "Date")
			grp, _ = bookmarksDateButton.GetGroup()
			bookmarksDateButton.Connect("activate", func() {
				self.config.Bookmarks.Absolute = true
				self.config.Bookmarks.Format = "2006/01/02 15:04:05"
				self.data.LoadBookmarks()
			})
			formatMenu.Append(bookmarksDateButton)

			bookmarksTimeButton, _ := gtk.RadioMenuItemNewWithLabel(grp, "Time")
			grp, _ = bookmarksTimeButton.GetGroup()
			bookmarksTimeButton.Connect("activate", func() {
				self.config.Bookmarks.Absolute = true
				self.config.Bookmarks.Format = "15:04:05"
				self.data.LoadBookmarks()
			})
			formatMenu.Append(bookmarksTimeButton)

			bookmarksOffsetButton, _ := gtk.RadioMenuItemNewWithLabel(grp, "Offset")
			grp, _ = bookmarksOffsetButton.GetGroup()
			bookmarksOffsetButton.Connect("activate", func() {
				self.config.Bookmarks.Absolute = false
				self.config.Bookmarks.Format = "04:05"
				self.data.LoadBookmarks()
			})
			formatMenu.Append(bookmarksOffsetButton)

			return formatMenu
		}())
		viewMenu.Add(formatButton)

		return viewMenu
	}())
	menuBar.Append(viewButton)

	/*
		analyseButton, _ := gtk.MenuItemNewWithLabel("Analyse")
		analyseButton.SetSubmenu(func() *gtk.Menu {
			analyseMenu, _ := gtk.MenuNew()

			timeChartButton, _ := gtk.MenuItemNewWithLabel("Time Chart")
			analyseMenu.Append(timeChartButton)

			return analyseMenu
		}())
		menuBar.Append(analyseButton)
	*/

	helpButton, _ := gtk.MenuItemNewWithLabel("Help")
	helpButton.SetSubmenu(func() *gtk.Menu {
		helpMenu, _ := gtk.MenuNew()

		aboutButton, _ := gtk.MenuItemNewWithLabel("About")
		aboutButton.Connect("activate", func(btn *gtk.MenuItem) {
			icon, _ := gdk.PixbufNewFromFile("data/context-name.svg")
			abt, _ := gtk.AboutDialogNew()
			abt.SetLogo(icon)
			abt.SetProgramName(self.name)
			abt.SetVersion(common.VERSION)
			abt.SetCopyright("(c) 2011-2014 Shish")
			abt.SetLicense(common.LICENSE)
			abt.SetWrapLicense(true)
			abt.SetWebsite("http://code.shishnet.org/context")
			//abt.SetAuthors([]string{"Shish <webmaster@shishnet.org>"})
			abt.Show()
		})
		helpMenu.Append(aboutButton)

		docButton, _ := gtk.MenuItemNewWithLabel("Documentation")
		// TODO
		/*
		   t.title("Context Documentation")
		   tx = Text(t)
		   tx.insert("0.0", data.README.replace("\r", ""))
		   tx.configure(state="disabled")
		   tx.focus_set()
		*/
		helpMenu.Append(docButton)

		return helpMenu
	}())
	menuBar.Append(helpButton)

	return menuBar
}

func (self *ContextViewer) __controlBox() *gtk.Grid {
	//-----------------------------------------------------------------

	gridTop, _ := gtk.GridNew()
	gridTop.SetOrientation(gtk.ORIENTATION_HORIZONTAL)

	l, _ := gtk.LabelNew(" Start ")
	gridTop.Add(l)

	// TODO: display as date, or offset, rather than unix timestamp?
	start, _ := gtk.SpinButtonNewWithRange(0, 0, 0.1)
	start.Connect("value-changed", func(sb *gtk.SpinButton) {
		if self.controls.active {
			log.Println("Settings: start =", sb.GetValue())
			self.SetStart(sb.GetValue())
			self.Update()
		}
	})
	gridTop.Add(start)
	self.controls.start = start

	l, _ = gtk.LabelNew("  Seconds ")
	gridTop.Add(l)

	length, _ := gtk.SpinButtonNewWithRange(MIN_SEC, MAX_SEC, 1.0)
	length.SetValue(self.config.Render.Length)
	length.Connect("value-changed", func(sb *gtk.SpinButton) {
		if self.controls.active {
			log.Println("Settings: len =", sb.GetValue())
			self.SetLength(sb.GetValue())
			self.Update()
		}
	})
	gridTop.Add(length)
	self.controls.length = length

	l, _ = gtk.LabelNew("  Pixels Per Second ")
	gridTop.Add(l)

	scale, _ := gtk.SpinButtonNewWithRange(MIN_PPS, MAX_PPS, 10.0)
	scale.SetValue(self.config.Render.Scale)
	scale.Connect("value-changed", func(sb *gtk.SpinButton) {
		if self.controls.active {
			log.Println("Settings: scale =", sb.GetValue())
			self.SetScale(sb.GetValue())
			self.canvas.QueueDraw()
		}
	})
	gridTop.Add(scale)
	self.controls.scale = scale

	//-----------------------------------------------------------------
	/*
		gridBot, _ := gtk.GridNew()
		gridBot.SetOrientation(gtk.ORIENTATION_HORIZONTAL)

		l, _ = gtk.LabelNew(" Cutoff (ms) ")
		gridBot.Add(l)

		cutoff, _ := gtk.SpinButtonNewWithRange(0, 1000, 10.0)
		cutoff.SetValue(self.config.Render.Cutoff * 1000)
		cutoff.Connect("value-changed", func(sb *gtk.SpinButton) {
			log.Println("Settings: cutoff =", sb.GetValue())
			self.config.Render.Cutoff = sb.GetValue() / 1000
			self.Update()
		})
		gridBot.Add(cutoff)

		l, _ = gtk.LabelNew("  Coalesce (ms) ")
		gridBot.Add(l)

		coalesce, _ := gtk.SpinButtonNewWithRange(0, 1000, 10.0)
		coalesce.SetValue(self.config.Render.Coalesce * 1000)
		coalesce.Connect("value-changed", func(sb *gtk.SpinButton) {
			log.Println("Settings: coalesce =", sb.GetValue())
			self.config.Render.Coalesce = sb.GetValue() / 1000
			self.Update()
		})
		gridBot.Add(coalesce)

		renderButton, _ := gtk.ButtonNewWithLabel("Render!")
		renderButton.Connect("clicked", func(sb *gtk.Button) {
			self.Update()
		})
		gridBot.Add(renderButton)
	*/
	//-----------------------------------------------------------------

	grid, _ := gtk.GridNew()
	grid.SetOrientation(gtk.ORIENTATION_VERTICAL)
	grid.Add(gridTop)
	//grid.Add(gridBot)

	return grid
}

func (self *ContextViewer) __bookmarks() *gtk.Grid {
	grid, _ := gtk.GridNew()

	// TODO: bookmark filter / search?
	// http://www.mono-project.com/GtkSharp_TreeView_Tutorial
	self.data.Bookmarks, _ = gtk.ListStoreNew(glib.TYPE_DOUBLE, glib.TYPE_STRING)

	// TODO: have SetStart affect this
	bookmarkScrollPane, _ := gtk.ScrolledWindowNew(nil, nil)
	bookmarkScrollPane.SetSizeRequest(250, 200)
	bookmarkView, _ := gtk.TreeViewNewWithModel(self.data.Bookmarks)
	bookmarkView.SetVExpand(true)
	bookmarkView.Connect("row-activated", func(bv *gtk.TreeView, path *gtk.TreePath, column *gtk.TreeViewColumn) {
		iter, _ := self.data.Bookmarks.GetIter(path)
		gvalue, _ := self.data.Bookmarks.GetValue(iter, 0)
		value, _ := gvalue.GoValue()
		fvalue := value.(float64)
		log.Printf("Nav: bookmark %.2f\n", fvalue)
		self.SetStart(fvalue)
		self.Update()
	})
	bookmarkScrollPane.Add(bookmarkView)
	grid.Attach(bookmarkScrollPane, 0, 0, 5, 1)

	renderer, _ := gtk.CellRendererTextNew()
	column, _ := gtk.TreeViewColumnNewWithAttribute("Bookmarks", renderer, "text", 1)
	bookmarkView.AppendColumn(column)

	l, _ := gtk.ButtonNewWithLabel("<<")
	l.Connect("clicked", func() {
		log.Println("Nav: Start")
		self.SetStart(self.data.LogStart)
		self.Update()
	})
	grid.Attach(l, 0, 1, 1, 1)

	l, _ = gtk.ButtonNewWithLabel("<")
	l.Connect("clicked", func() {
		log.Println("Nav: Prev")
		self.SetStart(self.data.GetLatestBookmarkBefore(self.config.Render.Start))
		self.Update()
	})
	grid.Attach(l, 1, 1, 1, 1)

	//l, _ = gtk.ButtonNewWithLabel(" ")
	//grid.Attach(l, 2, 1, 1, 1)

	l, _ = gtk.ButtonNewWithLabel(">")
	l.Connect("clicked", func() {
		log.Println("Nav: Next")
		self.SetStart(self.data.GetEarliestBookmarkAfter(self.config.Render.Start))
		self.Update()
	})
	grid.Attach(l, 3, 1, 1, 1)

	l, _ = gtk.ButtonNewWithLabel(">>")
	l.Connect("clicked", func() {
		log.Println("Nav: End")
		self.SetStart(self.data.LogEnd - self.config.Render.Length)
		self.Update()
	})
	grid.Attach(l, 4, 1, 1, 1)

	return grid
}

func (self *ContextViewer) __canvas() *gtk.Grid {
	grid, _ := gtk.GridNew()

	canvasScrollPane, _ := gtk.ScrolledWindowNew(nil, nil)
	canvasScrollPane.SetSizeRequest(200, 200)

	canvas, _ := gtk.DrawingAreaNew()
	canvas.SetHExpand(true)
	canvas.SetVExpand(true)
	canvas.Connect("draw", func(widget *gtk.DrawingArea, cr *cairo.Context) {
		width := int(self.config.Render.Scale * self.config.Render.Length)
		height := int(HEADER_HEIGHT + len(self.data.Threads)*BLOCK_HEIGHT*self.config.Render.MaxDepth)
		widget.SetSizeRequest(width, height)
		self.RenderBase(cr)
		self.RenderData(cr)
	})
	canvas.AddEvents(gdk.BUTTON_PRESS_MASK | gdk.BUTTON_RELEASE_MASK | gdk.SCROLL_MASK)
	canvas.Connect("button-press-event", func(widget *gtk.DrawingArea, evt *gdk.Event) {
		var x, y float64
		evt.GetCoords(&x, &y)
		event := self.getEventAt(x, y)

		if event != nil {
			viewWidth := 500.0 // FIXME
			pps := viewWidth / event.Length()
			self.SetScale(pps)

			// TODO: move the view so that the selected (item x1 = left edge of screen + padding)
			//startFrac := (event.StartTime - self.config.Render.Start) / self.config.Render.Length
			//adj := gtk.AdjustmentNew()
			//canvasScrollPane.SetHAdjustment(adj)

			self.canvas.QueueDraw()
		}
	})
	canvas.Connect("scroll-event", func(widget *gtk.DrawingArea, evt *gdk.Event) {
		var x, y float64
		evt.GetCoords(&x, &y)
		log.Println("Grid scroll", x, y)
		// TODO: mouse wheel zoom
	/*
	   canvas.bind("<4>", lambda e: self.scale_view(e, 1.0 * 1.1))
	   canvas.bind("<5>", lambda e: self.scale_view(e, 1.0 / 1.1))

	   # in windows, mouse wheel events always go to the root window o_O
	   self.master.bind("<MouseWheel>", lambda e: self.scale_view(
	       e, ((1.0 * 1.1) if e.delta > 0 else (1.0 / 1.1))
	   ))
	*/
	})

	canvasScrollPane.Add(canvas)
	grid.Add(canvasScrollPane)

	self.canvas = canvas

	return grid
}

func (self *ContextViewer) __scrubber() *gtk.Grid {
	var clickStart, clickEnd float64

	grid, _ := gtk.GridNew()

	canvas, _ := gtk.DrawingAreaNew()
	canvas.SetSizeRequest(200, SCRUBBER_HEIGHT)
	canvas.SetHExpand(true)
	// TODO: render at actual size
	//canvas.Connect("size-allocate", func(widget *gtk.DrawingArea, alloc *gtk.Allocation) {
	//})h
	canvas.Connect("draw", func(widget *gtk.DrawingArea, cr *cairo.Context) {
		//GtkAllocation* alloc = g_new(GtkAllocation, 1);
		//gtk_widget_get_allocation(widget, alloc);
		//printf("widget size is currently %dx%d\n",alloc->width, alloc->height);
		//g_free(alloc);
		//width, _ := widget.GetSizeRequest()
		self.RenderScrubber(cr, 500.0)
	})
	canvas.AddEvents(gdk.BUTTON_PRESS_MASK | gdk.BUTTON_RELEASE_MASK)
	canvas.Connect("button-press-event", func(widget *gtk.DrawingArea, evt *gdk.Event) {
		var x, y float64
		evt.GetCoords(&x, &y)
		width := 500.0
		clickStart = x / width
	})
	canvas.Connect("button-release-event", func(widget *gtk.DrawingArea, evt *gdk.Event) {
		var x, y float64
		evt.GetCoords(&x, &y)
		width := 500.0
		clickEnd = x / width

		if clickStart > clickEnd {
			clickStart, clickEnd = clickEnd, clickStart
		}

		start := self.data.LogStart + (self.data.LogEnd-self.data.LogStart)*clickStart
		length := (self.data.LogEnd - self.data.LogStart) * (clickEnd - clickStart)

		log.Printf("Nav: scrubbing to %.2f + %.2f\n", start, length)

		// if we've dragged rather than clicking, set render length to drag length
		if clickEnd-clickStart > 0.01 { // more than 1% of the scrubber's width
			self.SetLength(length)
		}
		self.SetStart(start)
		self.Update()
	})
	grid.Add(canvas)

	self.scrubber = canvas

	return grid
}

func (self *ContextViewer) getEventAt(x, y float64) *viewer.Event {
	yRel := y - float64(HEADER_HEIGHT)
	threadID := int(yRel / float64(BLOCK_HEIGHT * self.config.Render.MaxDepth))
	//thread := self.data.Threads[threadID]
	depth := (int(yRel) % (BLOCK_HEIGHT * self.config.Render.MaxDepth)) / BLOCK_HEIGHT

	width := self.config.Render.Scale * self.config.Render.Length
	ts := self.config.Render.Start + ((x / width) * self.config.Render.Length)
	//log.Printf("Click is thread %d depth %d at timestamp %.2f\n", threadID, depth, ts)

	// TODO: binary search? Events should be in startDate order
	for _, event := range self.data.Data {
		if event.StartTime < ts && event.EndTime > ts && event.ThreadID == threadID && event.Depth == depth {
			log.Printf("Clicked on event '%s'\n", event.Text())
			return &event
		}
	}

	log.Println("Clicked on no event")
	return nil
}

func (self *ContextViewer) SetStatus(text string) {
	if text != "" {
		log.Println(text)
	}
	self.status.Pop(0) // RemoveAll?
	self.status.Push(0, text)
}

func (self *ContextViewer) ShowError(title, text string) {
	log.Printf("%s: %s\n", title, text)
	dialog := gtk.MessageDialogNewWithMarkup(
		self.master,
        gtk.DIALOG_DESTROY_WITH_PARENT,
        gtk.MESSAGE_ERROR,
        gtk.BUTTONS_CLOSE,
        text)
	dialog.SetTitle(title)
	dialog.Show()
}

func (self *ContextViewer) SetStart(ts float64) {
	self.controls.active = false
	defer func() { self.controls.active = true }()

	// TODO: highlight the first bookmark which is before or equal to RenderStart
	if ts >= self.data.LogStart && ts <= self.data.LogEnd {
		// If we go over the end of the log, step back a bit.
		// Actually, that breaks "the bookmark is at the left edge of the screen"
		//if ts + self.config.Render.Length > self.data.LogEnd {
		//	ts = self.data.LogEnd - self.config.Render.Len
		//}

		self.controls.start.SetValue(ts)
		self.config.Render.Start = ts
		self.scrubber.QueueDraw()
	}
}

func (self *ContextViewer) SetLength(length float64) {
	self.controls.active = false
	defer func() { self.controls.active = true }()

	self.controls.length.SetValue(length)
	self.config.Render.Length = length
	self.scrubber.QueueDraw()
}

func (self *ContextViewer) SetScale(scale float64) {
	self.controls.active = false
	defer func() { self.controls.active = true }()

	if scale < MIN_PPS { scale = MIN_PPS }
	if scale > MAX_PPS { scale = MAX_PPS }

	self.controls.scale.SetValue(scale)
	self.config.Render.Scale = scale
	//self.canvas.QueueDraw()
}

func (self *ContextViewer) Update() {
	// free old data
	self.data.Data = []viewer.Event{}
	// TODO: reset canvas scroll position
	self.canvas.QueueDraw()

	/*go*/ func() {
		self.data.LoadEvents(
			self.config.Render.Start, self.config.Render.Length,
			self.config.Render.Coalesce, self.config.Render.Cutoff,
			self.SetStatus)
		self.canvas.QueueDraw()
	}()
}

/**********************************************************************
* Open File
**********************************************************************/

func (self *ContextViewer) LoadFile(givenFile string) {
	databaseFile, err := self.data.LoadFile(givenFile, self.SetStatus, self.config)
	if err != nil {
		self.ShowError("Error", fmt.Sprintf("Error loading '%s':\n%s", givenFile, err))
		return
	}

	// update title and scrubber, as those are ~instant
	self.controls.active = false
	self.master.SetTitle(self.name + ": " + databaseFile)
	self.controls.start.SetRange(self.data.LogStart, self.data.LogEnd)
	self.controls.active = true
	self.scrubber.QueueDraw()

	// render canvas with empty data first, then load the data
	self.canvas.QueueDraw()
	self.SetStart(self.data.LogStart)
	self.Update()
}

/**********************************************************************
* Rendering
**********************************************************************/

func (self *ContextViewer) RenderScrubber(cr *cairo.Context, width float64) {
	cr.SetSourceRGB(1, 1, 1)
	cr.Paint()

	activityPeak := 0
	for _, el := range self.data.Summary {
		if el > activityPeak {
			activityPeak = el
		}
	}

	length := float64(len(self.data.Summary))
	for n, el := range self.data.Summary {
		fraction := float64(el) / float64(activityPeak)
		cr.SetSourceRGB(fraction, 1.0-fraction, 0.0)
		cr.Rectangle(float64(n)/length*width, 0, width/length, SCRUBBER_HEIGHT)
		cr.Fill()
	}

	cr.SetSourceRGB(0, 0, 0)
	cr.SetLineWidth(1.0)
	cr.Rectangle(0.5, 0.5, width-1, SCRUBBER_HEIGHT-1)
	cr.Stroke()

	if self.data.LogEnd == self.data.LogStart { // only one event in the log o_O?
		return
	}

	LogLength := self.data.LogEnd - self.data.LogStart

	// arrow
	start_rel := self.config.Render.Start - self.data.LogStart
	start := math.Floor((start_rel / LogLength) * width)

	end_rel := (self.config.Render.Start + self.config.Render.Length) - self.data.LogStart
	end := math.Floor((end_rel / LogLength) * width)

	line := func(x1, y1, x2, y2 float64) {
		cr.MoveTo(x1+0.5, y1+0.5)
		cr.LineTo(x2+0.5, y2+0.5)
		cr.Stroke()
	}

	// left edge
	line(start, 1, start, SCRUBBER_HEIGHT)
	line(start, SCRUBBER_HEIGHT/2, start+5, 15)
	line(start, SCRUBBER_HEIGHT/2, start+5, 5)

	// right edge
	line(end, 1, end, SCRUBBER_HEIGHT)
	line(end, SCRUBBER_HEIGHT/2, end-5, 15)
	line(end, SCRUBBER_HEIGHT/2, end-5, 5)

	// join
	line(start, SCRUBBER_HEIGHT/2, end, SCRUBBER_HEIGHT/2)
}

func (self *ContextViewer) RenderBase(cr *cairo.Context) {
	// common settings
	cr.SetLineWidth(1.0)
	cr.SelectFontFace("sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(10)

	line := func(x1, y1, x2, y2 float64) {
		cr.MoveTo(x1+0.5, y1+0.5)
		cr.LineTo(x2+0.5, y2+0.5)
		cr.Stroke()
	}

	width := self.config.Render.Scale * self.config.Render.Length
	height := float64(HEADER_HEIGHT + len(self.data.Threads)*self.config.Render.MaxDepth*BLOCK_HEIGHT)

	// blank canvas
	cr.SetSourceRGB(1, 1, 1)
	cr.Paint()

	// draw vertical bars (time)
	for x := 0.0; x < width; x += 100.0 {
		cr.SetSourceRGB(0.8, 0.8, 0.8)
		line(x, 0, x, height)

		cr.SetSourceRGB(0.4, 0.4, 0.4)
		cr.MoveTo(x, HEADER_HEIGHT*0.70)
		cr.ShowText(fmt.Sprintf(" +%.4f", float64(x)/width*self.config.Render.Length))
	}

	// draw horizontal bars (thread)
	cr.SetSourceRGB(0.75, 0.75, 0.75)
	cr.SetLineWidth(1.0)
	for n, _ := range self.data.Threads {
		y := float64(HEADER_HEIGHT + self.config.Render.MaxDepth*BLOCK_HEIGHT*n)
		line(0, y, width, y)

		cr.SetSourceRGB(0.4, 0.4, 0.4)
		cr.MoveTo(3.0, float64(HEADER_HEIGHT+self.config.Render.MaxDepth*BLOCK_HEIGHT*(n+1)-5))
		cr.ShowText(self.data.Threads[n])
	}
}

func (self *ContextViewer) RenderData(cr *cairo.Context) {
	defer self.SetStatus("")

	cr.SelectFontFace("sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(10)

	_rs := self.config.Render.Start
	_rc := self.config.Render.Cutoff
	_sc := self.config.Render.Scale

	shown := 0
	for _, event := range self.data.Data {
		thread_idx := event.ThreadID

		switch {
		case event.StartType == "START":
			if (event.EndTime-event.StartTime)*1000 < _rc {
				continue
			}
			if event.Depth >= self.config.Render.MaxDepth {
				continue
			}
			shown += 1
			if common.DEMO && shown == 500 {
				self.ShowError("Demo Limit", "The evaluation build is limited to showing 500 events at a time, so rendering has stopped")
				return
			}
			self.ShowEvent(cr, &event, _rs, _sc, thread_idx)

		case event.StartType == "BMARK":
			if self.config.Render.Bookmarks {
				self.ShowBookmark(cr, &event, _rs, _sc)
			}

		case event.StartType == "LOCKW" || event.StartType == "LOCKA":
			self.ShowLock(cr, &event, _rs, _sc, thread_idx)
		}
	}
}

func (self *ContextViewer) ShowEvent(cr *cairo.Context, event *viewer.Event, offset_time, scale_factor float64, thread int) {
	ok := event.EndType == "ENDOK"

	start_px := (event.StartTime - offset_time) * scale_factor
	length_px := event.Length() * scale_factor
	depth_px := float64(HEADER_HEIGHT + (thread * (self.config.Render.MaxDepth * BLOCK_HEIGHT)) + (event.Depth * BLOCK_HEIGHT))

	if ok {
		cr.SetSourceRGB(0.8, 1.0, 0.8)
	} else {
		cr.SetSourceRGB(1.0, 0.8, 0.8)
	}
	cr.Rectangle(start_px, depth_px, length_px, BLOCK_HEIGHT)
	cr.Fill()

	if ok {
		cr.SetSourceRGB(0.3, 0.5, 0.3)
	} else {
		cr.SetSourceRGB(0.5, 0.3, 0.3)
	}
	cr.Rectangle(math.Floor(start_px)+0.5, depth_px+0.5, math.Floor(length_px), BLOCK_HEIGHT)
	cr.Stroke()

	cr.Save()
	cr.Rectangle(start_px, depth_px, math.Max(0, length_px-5), BLOCK_HEIGHT) // length-5 = have padding
	cr.Clip()
	cr.MoveTo(start_px+5, depth_px+BLOCK_HEIGHT*0.70)
	cr.ShowText(event.Text())
	cr.Restore()
}

func (self *ContextViewer) ShowLock(cr *cairo.Context, event *viewer.Event, offset_time, scale_factor float64, thread int) {
	start_px := (event.StartTime - offset_time) * scale_factor
	length_px := event.Length() * scale_factor

	if event.StartType == "LOCKW" {
		cr.SetSourceRGB(1.0, 0.85, 0.85)
	} else {
		cr.SetSourceRGB(0.85, 0.85, 1.0)
	}
	cr.Rectangle(
		start_px, float64(HEADER_HEIGHT+thread*self.config.Render.MaxDepth*BLOCK_HEIGHT),
		length_px, float64(self.config.Render.MaxDepth*BLOCK_HEIGHT),
	)
	cr.Fill()

	cr.SetSourceRGB(0.5, 0.5, 0.5)
	cr.Save()
	cr.Rectangle(
		start_px, float64(HEADER_HEIGHT+thread*self.config.Render.MaxDepth*BLOCK_HEIGHT),
		length_px-5, float64(self.config.Render.MaxDepth*BLOCK_HEIGHT),
	)
	cr.Clip()
	cr.MoveTo(start_px+5, float64(HEADER_HEIGHT+(thread+1)*self.config.Render.MaxDepth*BLOCK_HEIGHT)-5)
	cr.ShowText(event.Text())
	cr.Restore()
}

func (self *ContextViewer) ShowBookmark(cr *cairo.Context, event *viewer.Event, offset_time, scale_factor float64) {
	start_px := math.Floor((event.StartTime - offset_time) * scale_factor)
	height := float64(HEADER_HEIGHT + len(self.data.Threads)*self.config.Render.MaxDepth*BLOCK_HEIGHT)

	cr.SetSourceRGB(1.0, 0.5, 0.0)
	cr.MoveTo(start_px+0.5, HEADER_HEIGHT)
	cr.LineTo(start_px+0.5, height)
	cr.Stroke()

	cr.MoveTo(start_px+5, height-5)
	cr.ShowText(event.Text())
}

/*
	//	tip := fmt.Sprintf("%dms @%dms: %s\n%s",
	//	   (event.EndTime - event.StartTime) * 1000,
	//	   (event.StartTime - offset_time) * 1000,
	//	   event.start_location, event.Text())

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
*/

/**********************************************************************
* Main
**********************************************************************/

func main() {
	// add ./ to path so context-compiler can be found
	path := os.Getenv("PATH")
	newPath := filepath.Dir(os.Args[0]) + ":" + path
	_ = os.Setenv("PATH", newPath)

	var geometry = flag.String("g", "800x600", "Set window geometry")
	flag.Parse()

	var w, h int
	if geometry != nil {
		parts := strings.SplitN(*geometry, "x", 2)
		w, _ = strconv.Atoi(parts[0])
		h, _ = strconv.Atoi(parts[1])
	}

	var filename *string
	if len(flag.Args()) >= 1 {
		filename = &flag.Args()[0]
	}

	gtk.Init(nil)

	cv := ContextViewer{}
	cv.Init(filename, Geometry{w, h})

	gtk.Main()
}
