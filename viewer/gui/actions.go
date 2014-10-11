package gui

import (
	"../event"
	"fmt"
	"github.com/conformal/gotk3/gtk"
	"github.com/conformal/gotk3/glib"
)

/**********************************************************************
* Open File
**********************************************************************/

func (self *ContextViewer) LoadFile(givenFile string) {
	// open .cbin file, instant if cbin is ready, slow if .ctxt needs compiling
	go func() {
		databaseFile, err := self.data.OpenFile(givenFile, self.config)
		if err != nil {
			self.showError("Error", fmt.Sprintf("Error loading '%s':\n%s", givenFile, err))
			return
		}

		self.data.LoadSettings()
		glib.IdleAdd(func() {
			// update title and scrubber, as those are ~instant
			self.controls.active = false
			self.master.SetTitle(self.name + ": " + databaseFile)
			self.controls.start.SetRange(
				self.data.LogStart,
				self.data.LogEnd)
				//float64(int(self.data.LogStart*10))/10,
				//float64(int(self.data.LogEnd*10) + 1)/10)
			self.controls.active = true
		})

		self.data.LoadSummary()
		glib.IdleAdd(func() {
			self.scrubber.QueueDraw()
		})

		self.data.LoadBookmarks()
		glib.IdleAdd(func() {
			self.setStatus("Rendering bookmarks")
			// convert data.Bookmarks to self.bookmarks
			self.bookmarks.Clear()
			for _, bookmark := range(self.data.Bookmarks) {
				itemPtr := self.bookmarks.Append()
				self.bookmarks.Set(itemPtr, []int{0, 1}, []interface{}{bookmark.Time, bookmark.Text})
			}
		})

		self.data.LoadThreads()
		glib.IdleAdd(func() {
			// render canvas with empty data first, then load the data
			self.redraw()
			self.SetStart(self.data.LogStart)
			self.Update()
		})
	}()
}

func (self *ContextViewer) SetStart(ts float64) {
	self.controls.active = false
	defer func() { self.controls.active = true }()

	if ts >= self.data.LogStart && ts <= self.data.LogEnd {
		// If we go over the end of the log, step back a bit.
		// Actually, that breaks "the bookmark is at the left edge of the screen"
		//if ts + self.config.Render.Length > self.data.LogEnd {
		//	ts = self.data.LogEnd - self.config.Render.Len
		//}

		itemNum := -1
		for iter, valid := self.bookmarks.GetIterFirst(); valid; valid = self.bookmarks.IterNext(iter) {
			gTimestamp, _ := self.bookmarks.GetValue(iter, 0)
			timestamp, _ := gTimestamp.GoValue()

			if timestamp.(float64) >= ts {
				break
			}

			itemNum++
		}
		if itemNum < 0 {
			itemNum = 0
		}
		selection, _ := self.controls.bookmarks.GetSelection()
		path, _ := gtk.TreePathNewFromString(fmt.Sprintf("%d", itemNum))
		selection.SelectPath(path)

		self.controls.start.SetValue(ts)
		self.config.Render.Start = ts

		adj := self.canvasScroll.GetHAdjustment()
		adj.SetValue(0)

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

	if scale < MIN_PPS {
		scale = MIN_PPS
	}
	if scale > MAX_PPS {
		scale = MAX_PPS
	}

	self.controls.scale.SetValue(scale)
	self.config.Render.Scale = scale
	//self.redraw()
}

func (self *ContextViewer) SetDepth(depth int) {
	self.controls.active = false
	defer func() { self.controls.active = true }()

	if depth < MIN_DEPTH {
		depth = MIN_DEPTH
	}
	if depth > MAX_DEPTH {
		depth = MAX_DEPTH
	}

	self.controls.depth.SetValue(float64(depth))
	self.config.Render.Depth = depth
	//self.redraw()
}

// Load new event data based on render settings
func (self *ContextViewer) Update() {
	// free old data
	self.data.Data = []event.Event{}
	self.redraw()

	go func() {
		self.data.LoadEvents(
			self.config.Render.Start, self.config.Render.Length,
			self.config.Render.Coalesce, self.config.Render.Cutoff,
			)
		glib.IdleAdd(func() {
			self.redraw()
		})
	}()
}

func (self *ContextViewer) redraw() {
	self.buffer = nil
	self.canvas.QueueDraw()
}

func (self *ContextViewer) getEventAt(x, y float64) *event.Event {
	row_count := len(self.data.VisibleThreadIDs)
	row_depth := BLOCK_HEIGHT*self.config.Render.Depth
	total_height := float64(HEADER_HEIGHT+row_depth*row_count)
	if y < HEADER_HEIGHT || y > total_height {
		return nil
	}

	yRel := y - float64(HEADER_HEIGHT)
	threadIndex := int(yRel / float64(BLOCK_HEIGHT*self.config.Render.Depth))
	depth := (int(yRel) % (BLOCK_HEIGHT * self.config.Render.Depth)) / BLOCK_HEIGHT

	width := self.config.Render.Scale * self.config.Render.Length
	ts := self.config.Render.Start + ((x / width) * self.config.Render.Length)
	//log.Printf("Click is thread %d depth %d at timestamp %.2f\n", threadIndex, depth, ts)

	// binary search? Events should be in startDate order...
	// though different threads are all mixed together.
	// binary search for first event with startTime > mousePos,
	// then iterate backwards skipping over unrelated threads?
	for _, event := range self.data.Data {
		if event.StartTime < ts && event.EndTime > ts &&
			event.ThreadIndex == threadIndex && event.Depth == depth {
			return &event
		}
	}

	return nil
}
