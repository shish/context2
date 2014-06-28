package gui

import (
	"log"
	"os/user"
	"path/filepath"
	// local
	"../../common"
	"../config"
	"../data"
	"../event"
	// gtk
	"github.com/conformal/gotk3/cairo"
	"github.com/conformal/gotk3/gdk"
	"github.com/conformal/gotk3/glib"
	"github.com/conformal/gotk3/gtk"
)

/**********************************************************************
* Structs
**********************************************************************/

type Geometry struct {
	W int
	H int
}

type ContextViewer struct {
	name string
	// GUI
	master        *gtk.Window
	canvasScroll  *gtk.ScrolledWindow
	canvas        *gtk.DrawingArea
	buffer        *cairo.Surface
	scrubber      *gtk.DrawingArea
	status        *gtk.Statusbar
	bookmarkPanel *gtk.Grid
	renderSettings *gtk.Grid
	dataSettings *gtk.Grid
	configFile    string
	config        config.Config

	// data
	data        data.Data
	activeEvent *event.Event

	controls struct {
		active bool
		bookmarks *gtk.TreeView
		start  *gtk.SpinButton
		length *gtk.SpinButton
		scale  *gtk.SpinButton
		depth  *gtk.SpinButton
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
	master.SetDefaultSize(geometry.W, geometry.H)
	icon, err := gdk.PixbufNewFromFile("data/context-icon.svg")
	if err == nil {
		master.SetIcon(icon)
	}

	self.master = master

	usr, _ := user.Current()
	self.configFile = usr.HomeDir + "/.config/context2.cfg"
	err = self.config.Load(self.configFile)
	if err != nil {
		log.Printf("Error loading config file %s: %s\n", self.configFile, err)
	}

	master.Connect("destroy", self.Exit)

	menuBar := self.buildMenu()
	controls := self.buildControlBox()
	bookmarks := self.buildBookmarks()
	canvas := self.buildCanvas()
	scrubber := self.buildScrubber()

	status, err := gtk.StatusbarNew()
	if err != nil {
		log.Fatal("Unable to create label:", err)
	}

	pane, _ := gtk.PanedNew(gtk.ORIENTATION_HORIZONTAL)
	pane.Pack1(bookmarks, true, false)
	pane.Pack2(canvas, true, false)
	pane.SetPosition(200)

	grid, _ := gtk.GridNew()
	grid.SetOrientation(gtk.ORIENTATION_VERTICAL)
	grid.Add(menuBar)
	grid.Add(controls)
	grid.Add(pane)
	grid.Add(scrubber)
	grid.Add(status)
	master.Add(grid)

	self.bookmarkPanel = bookmarks
	self.status = status

	self.controls.active = true
	master.ShowAll()

	if !self.config.Gui.BookmarkPanel {
		self.bookmarkPanel.Hide()
	}
	if !self.config.Gui.RenderSettings {
		self.renderSettings.Hide()
	}
	if !self.config.Gui.DataSettings {
		self.dataSettings.Hide()
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

func (self *ContextViewer) buildMenu() *gtk.MenuBar {
	menuBar, _ := gtk.MenuBarNew()

	fileButton, _ := gtk.MenuItemNewWithLabel("File")
	fileButton.SetSubmenu(func() *gtk.Menu {
		fileMenu, _ := gtk.MenuNew()

		openButton, _ := gtk.MenuItemNewWithLabel("Open .ctxt / .cbin")
		openButton.Connect("activate", func() {
			/*
					// TODO: filter by extension in file open dialog box
				   filetypes=[
					   ("All Supported Types", "*.ctxt *.cbin"),
					   ("Context Text", "*.ctxt"),
					   ("Context Binary", "*.cbin")
				   ],
				   initialdir=self._last_log_dir
			*/
			dialog, _ := gtk.FileChooserDialogNew2(
				"Open File", self.master, gtk.FILE_CHOOSER_ACTION_OPEN,
				"Cancel", gtk.RESPONSE_CANCEL,
				"Open", gtk.RESPONSE_ACCEPT)
			if (gtk.ResponseType)(dialog.Run()) == gtk.RESPONSE_ACCEPT {
				filename := dialog.GetFilename()
				self.config.Gui.LastLogDir = filepath.Dir(filename)
				self.LoadFile(filename)
			}

			dialog.Destroy()
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

		showRenderSettingsButton, _ := gtk.CheckMenuItemNewWithLabel("Show Render Settings")
		showRenderSettingsButton.SetActive(self.config.Gui.RenderSettings)
		showRenderSettingsButton.Connect("activate", func() {
			self.config.Gui.RenderSettings = showRenderSettingsButton.GetActive()
			if self.config.Gui.BookmarkPanel {
				self.renderSettings.Show()
			} else {
				self.renderSettings.Hide()
			}
		})
		viewMenu.Append(showRenderSettingsButton)

		showDataSettingsButton, _ := gtk.CheckMenuItemNewWithLabel("Show Data Settings")
		showDataSettingsButton.SetActive(self.config.Gui.DataSettings)
		showDataSettingsButton.Connect("activate", func() {
			self.config.Gui.DataSettings = showDataSettingsButton.GetActive()
			if self.config.Gui.BookmarkPanel {
				self.dataSettings.Show()
			} else {
				self.dataSettings.Hide()
			}
		})
		viewMenu.Append(showDataSettingsButton)

		sep1, _ := gtk.SeparatorMenuItemNew()
		viewMenu.Append(sep1)

		showBookmarksButton, _ := gtk.CheckMenuItemNewWithLabel("Render Bookmarks")
		showBookmarksButton.SetActive(self.config.Render.Bookmarks)
		showBookmarksButton.Connect("activate", func() {
			self.config.Render.Bookmarks = showBookmarksButton.GetActive()
			self.redraw()
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
			abt, _ := gtk.AboutDialogNew()

			logo, err := gdk.PixbufNewFromFileAtScale("data/context-name.svg", 300, 200, true)
			if err == nil {
				abt.SetLogo(logo)
			}

			abt.SetProgramName(self.name)
			abt.SetVersion(common.VERSION)
			abt.SetCopyright("(c) 2011-2014 Shish")
			abt.SetLicense(common.LICENSE)
			abt.SetWrapLicense(true)
			abt.SetWebsite("http://code.shishnet.org/context")
			//abt.SetAuthors([]string{"Shish <webmaster@shishnet.org>"})

			icon, err := gdk.PixbufNewFromFile("data/tools-icon.svg")
			if err == nil {
				abt.SetIcon(icon)
			}

			abt.Run()
			abt.Destroy()
		})
		helpMenu.Append(aboutButton)

		docButton, _ := gtk.MenuItemNewWithLabel("Documentation")
		docButton.Connect("activate", func(btn *gtk.MenuItem) {
			dialog := gtk.MessageDialogNew(
				self.master,
				gtk.DIALOG_DESTROY_WITH_PARENT,
				gtk.MESSAGE_INFO,
				gtk.BUTTONS_CLOSE,
				"If you need any help, contact webmaster@shishnet.org")
			dialog.SetTitle(NAME + " Documentation")

			ca, _ := dialog.GetContentArea()
			//ca.RemoveAll()

			tt, _ := gtk.TextTagTableNew()
			buffer, _ := gtk.TextBufferNew(tt)
			buffer.SetText(common.README)
			tv, _ := gtk.TextViewNewWithBuffer(buffer)
			tv.SetWrapMode(gtk.WRAP_WORD)
			tv.SetEditable(false)

			readmeScrollPane, _ := gtk.ScrolledWindowNew(nil, nil)
			readmeScrollPane.SetSizeRequest(600, 300)
			readmeScrollPane.Add(tv)
			ca.Add(readmeScrollPane)

			dialog.ShowAll()
			dialog.Run()
			dialog.Destroy()
		})
		helpMenu.Append(docButton)

		return helpMenu
	}())
	menuBar.Append(helpButton)

	return menuBar
}

func (self *ContextViewer) buildControlBox() *gtk.Grid {
	renderSettings, _ := gtk.GridNew()
	renderSettings.SetOrientation(gtk.ORIENTATION_HORIZONTAL)

	l, _ := gtk.LabelNew(" Start ")
	renderSettings.Add(l)

	// TODO: display as date, or offset, rather than unix timestamp?
	start, _ := gtk.SpinButtonNewWithRange(0, 0, 0.1)
	start.Connect("value-changed", func(sb *gtk.SpinButton) {
		if self.controls.active {
			log.Println("Settings: start =", sb.GetValue())
			self.SetStart(sb.GetValue())
			self.Update()
		}
	})
	renderSettings.Add(start)
	self.controls.start = start

	l, _ = gtk.LabelNew("  Seconds ")
	renderSettings.Add(l)

	length, _ := gtk.SpinButtonNewWithRange(MIN_SEC, MAX_SEC, 1.0)
	length.SetValue(self.config.Render.Length)
	length.Connect("value-changed", func(sb *gtk.SpinButton) {
		if self.controls.active {
			log.Println("Settings: len =", sb.GetValue())
			self.SetLength(sb.GetValue())
			self.Update()
		}
	})
	renderSettings.Add(length)
	self.controls.length = length

	l, _ = gtk.LabelNew("  Pixels Per Second ")
	renderSettings.Add(l)

	scale, _ := gtk.SpinButtonNewWithRange(MIN_PPS, MAX_PPS, 10.0)
	scale.SetValue(self.config.Render.Scale)
	scale.Connect("value-changed", func(sb *gtk.SpinButton) {
		if self.controls.active {
			log.Println("Settings: scale =", sb.GetValue())
			self.SetScale(sb.GetValue())
			self.redraw()
		}
	})
	renderSettings.Add(scale)
	self.controls.scale = scale

	l, _ = gtk.LabelNew("  Depth ")
	renderSettings.Add(l)

	depth, _ := gtk.SpinButtonNewWithRange(MIN_PPS, MAX_PPS, 1.0)
	depth.SetValue(float64(self.config.Render.Depth))
	depth.Connect("value-changed", func(sb *gtk.SpinButton) {
		if self.controls.active {
			log.Println("Settings: depth =", sb.GetValue())
			self.SetDepth(int(sb.GetValue()))
			self.redraw()
		}
	})
	renderSettings.Add(depth)
	self.controls.depth = depth

	//-----------------------------------------------------------------
	dataSettings, _ := gtk.GridNew()
	dataSettings.SetOrientation(gtk.ORIENTATION_HORIZONTAL)

	l, _ = gtk.LabelNew(" Cutoff ")
	dataSettings.Add(l)

	cutoff, _ := gtk.SpinButtonNewWithRange(0, 1, 0.001)
	cutoff.SetValue(self.config.Render.Cutoff)
	cutoff.Connect("value-changed", func(sb *gtk.SpinButton) {
		log.Println("Settings: cutoff =", sb.GetValue())
		self.config.Render.Cutoff = sb.GetValue()
		self.Update()
	})
	dataSettings.Add(cutoff)

	l, _ = gtk.LabelNew("  Coalesce ")
	dataSettings.Add(l)

	coalesce, _ := gtk.SpinButtonNewWithRange(0, 1, 0.001)
	coalesce.SetValue(self.config.Render.Coalesce)
	coalesce.Connect("value-changed", func(sb *gtk.SpinButton) {
		log.Println("Settings: coalesce =", sb.GetValue())
		self.config.Render.Coalesce = sb.GetValue()
		self.Update()
	})
	dataSettings.Add(coalesce)

	/*
	renderButton, _ := gtk.ButtonNewWithLabel("Render!")
	renderButton.Connect("clicked", func(sb *gtk.Button) {
		self.Update()
	})
	dataSettings.Add(renderButton)
	*/

	//-----------------------------------------------------------------

	self.renderSettings = renderSettings
	self.dataSettings = dataSettings

	grid, _ := gtk.GridNew()
	grid.SetOrientation(gtk.ORIENTATION_VERTICAL)
	grid.Add(renderSettings)
	grid.Add(dataSettings)

	return grid
}

func (self *ContextViewer) buildBookmarks() *gtk.Grid {
	grid, _ := gtk.GridNew()

	// TODO: bookmark filter / search?
	// http://www.mono-project.com/GtkSharp_TreeView_Tutorial
	self.data.Bookmarks, _ = gtk.ListStoreNew(glib.TYPE_DOUBLE, glib.TYPE_STRING)

	bookmarkScrollPane, _ := gtk.ScrolledWindowNew(nil, nil)
	bookmarkScrollPane.SetSizeRequest(250, 200)
	bookmarkScrollPane.SetHExpand(true)
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
	self.controls.bookmarks = bookmarkView
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

func (self *ContextViewer) buildCanvas() *gtk.Grid {
	grid, _ := gtk.GridNew()

	canvasScroll, _ := gtk.ScrolledWindowNew(nil, nil)
	canvasScroll.SetSizeRequest(200, 200)

	canvas, _ := gtk.DrawingAreaNew()
	canvas.SetHExpand(true)
	canvas.SetVExpand(true)
	canvas.AddEvents(
		gdk.BUTTON_PRESS_MASK | gdk.BUTTON_RELEASE_MASK |
			/*gdk.SCROLL_MASK |*/ gdk.POINTER_MOTION_MASK)
	canvas.Connect("draw", func(widget *gtk.DrawingArea, cr *cairo.Context) {
		width := int(self.config.Render.Scale * self.config.Render.Length)
		height := int(HEADER_HEIGHT + len(self.data.VisibleThreadIDs)*BLOCK_HEIGHT*self.config.Render.Depth)
		widget.SetSizeRequest(width, height)
		self.renderCanvas(cr, width, height)
	})
	canvas.Connect("damage-event", func(widget *gtk.DrawingArea, evt *gdk.Event) {
		//log.Println(evt.area)
		log.Println("exposed")
	})
	canvas.Connect("motion-notify-event", func(widget *gtk.DrawingArea, gevt *gdk.Event) {
		var x, y float64
		gevt.GetCoords(&x, &y)
		evt := self.getEventAt(x, y)
		if !event.CmpEvent(evt, self.activeEvent) {
			self.activeEvent = evt
			self.canvas.QueueDraw() // don't do a full redraw
		}
	})
	canvas.Connect("button-press-event", func(widget *gtk.DrawingArea, evt *gdk.Event) {
		var x, y float64
		evt.GetCoords(&x, &y)
		event := self.getEventAt(x, y)

		if event != nil {
			padding := 10.0

			pps := (float64(canvasScroll.GetAllocatedWidth()) - (padding * 2)) / event.Length()
			self.SetScale(pps)

			startPos := (event.StartTime-self.config.Render.Start)*self.config.Render.Scale - padding

			adj := canvasScroll.GetHAdjustment()
			adj.SetValue(startPos)

			self.redraw()
		}
	})
	canvas.Connect("scroll-event", func(widget *gtk.DrawingArea, evt *gdk.Event) {
		var x, y float64
		evt.GetCoords(&x, &y)
		log.Println("Grid scroll", x, y)
		// TODO: mouse wheel zoom?
		/*
			   canvas.bind("<4>", lambda e: self.scale_view(e, 1.0 * 1.1))
			   canvas.bind("<5>", lambda e: self.scale_view(e, 1.0 / 1.1))

			   # in windows, mouse wheel events always go to the root window o_O
			   self.master.bind("<MouseWheel>", lambda e: self.scale_view(
				   e, ((1.0 * 1.1) if e.delta > 0 else (1.0 / 1.1))
			   ))
		*/
	})

	canvasScroll.Add(canvas)
	grid.Add(canvasScroll)

	self.canvasScroll = canvasScroll
	self.canvas = canvas

	return grid
}

func (self *ContextViewer) buildScrubber() *gtk.Grid {
	var clickStart, clickEnd float64

	grid, _ := gtk.GridNew()

	canvas, _ := gtk.DrawingAreaNew()
	canvas.SetSizeRequest(200, SCRUBBER_HEIGHT)
	canvas.SetHExpand(true)
	canvas.AddEvents(gdk.BUTTON_PRESS_MASK | gdk.BUTTON_RELEASE_MASK)
	canvas.Connect("draw", func(widget *gtk.DrawingArea, cr *cairo.Context) {
		self.renderScrubber(cr, float64(widget.GetAllocatedWidth()))
	})
	canvas.Connect("button-press-event", func(widget *gtk.DrawingArea, evt *gdk.Event) {
		var x, y float64
		evt.GetCoords(&x, &y)
		width := float64(widget.GetAllocatedWidth())
		clickStart = x / width
	})
	canvas.Connect("button-release-event", func(widget *gtk.DrawingArea, evt *gdk.Event) {
		var x, y float64
		evt.GetCoords(&x, &y)
		width := float64(widget.GetAllocatedWidth())
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

func (self *ContextViewer) setStatus(text string) {
	if text != "" {
		log.Println(text)
	}
	self.status.Pop(0) // RemoveAll?
	self.status.Push(0, text)
}

func (self *ContextViewer) showError(title, text string) {
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
