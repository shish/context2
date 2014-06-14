package gui

import (
	"fmt"
	"../event"
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
	self.canvas.QueueDraw()
	self.SetStart(self.data.LogStart)
	self.Update()
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
	self.data.Data = []event.Event{}
	// TODO: reset canvas scroll position
	self.canvas.QueueDraw()

	/*go*/ func() {
		self.data.LoadEvents(
			self.config.Render.Start, self.config.Render.Length,
			self.config.Render.Coalesce, self.config.Render.Cutoff,
			self.setStatus)
		self.canvas.QueueDraw()
	}()
}

func (self *ContextViewer) getEventAt(x, y float64) *event.Event {
	yRel := y - float64(HEADER_HEIGHT)
	threadID := int(yRel / float64(BLOCK_HEIGHT * self.config.Render.MaxDepth))
	depth := (int(yRel) % (BLOCK_HEIGHT * self.config.Render.MaxDepth)) / BLOCK_HEIGHT

	width := self.config.Render.Scale * self.config.Render.Length
	ts := self.config.Render.Start + ((x / width) * self.config.Render.Length)
	//log.Printf("Click is thread %d depth %d at timestamp %.2f\n", threadID, depth, ts)

	// TODO: binary search? Events should be in startDate order
	for _, event := range self.data.Data {
		if (event.StartTime < ts && event.EndTime > ts &&
		event.ThreadID == threadID && event.Depth == depth) {
			return &event
		}
	}

	return nil
}