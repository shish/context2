package gui

import (
	"../event"
	"fmt"
	"github.com/conformal/gotk3/gtk"
)

/**********************************************************************
* Open File
**********************************************************************/

func (self *ContextViewer) LoadFile(givenFile string) {
	databaseFile, err := self.data.LoadFile(givenFile, self.setStatus, self.config)
	if err != nil {
		self.showError("Error", fmt.Sprintf("Error loading '%s':\n%s", givenFile, err))
		return
	}

	// update title and scrubber, as those are ~instant
	self.controls.active = false
	self.master.SetTitle(self.name + ": " + databaseFile)
	self.controls.start.SetRange(self.data.LogStart, self.data.LogEnd)
	self.controls.active = true
	self.scrubber.QueueDraw()

	// render canvas with empty data first, then load the data
	self.redraw()
	self.SetStart(self.data.LogStart)
	self.Update()
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
		for iter, valid := self.data.Bookmarks.GetIterFirst(); valid; valid = self.data.Bookmarks.IterNext(iter) {
			gTimestamp, _ := self.data.Bookmarks.GetValue(iter, 0)
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

	if depth < 1 {
		depth = 1
	}
	if depth > 20 {
		depth = 20
	}

	self.controls.depth.SetValue(float64(depth))
	self.config.Render.Depth = depth
	//self.redraw()
}

func (self *ContextViewer) Update() {
	// free old data
	self.data.Data = []event.Event{}
	self.redraw()

	/*go*/ func() {
		self.data.LoadEvents(
			self.config.Render.Start, self.config.Render.Length,
			self.config.Render.Coalesce, self.config.Render.Cutoff,
			self.setStatus)
		self.redraw()
	}()
}

func (self *ContextViewer) redraw() {
	self.buffer = nil
	self.canvas.QueueDraw()
}

func (self *ContextViewer) getEventAt(x, y float64) *event.Event {
	if y < HEADER_HEIGHT ||
		y > float64(HEADER_HEIGHT+(BLOCK_HEIGHT*self.config.Render.Depth)*len(self.data.VisibleThreadIDs)) {
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
