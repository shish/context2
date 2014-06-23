package gui

import (
	"fmt"
	"log"
	"math"
	// gtk
	"../../common"
	"../event"
	"github.com/conformal/gotk3/cairo"
)

/**********************************************************************
* Rendering
**********************************************************************/

func (self *ContextViewer) renderScrubber(cr *cairo.Context, width float64) {
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
		cr.Rectangle(math.Floor(float64(n)/length*width), 0, math.Floor(width/length)+1.0, SCRUBBER_HEIGHT)
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

func (self *ContextViewer) renderCanvas(cr *cairo.Context, width, height int) {
	if self.buffer == nil {
		log.Printf("Creating canvas buffer %dx%d\n", width, height)
		self.buffer = cairo.ImageSurfaceCreate(cairo.FORMAT_ARGB32, width, height)
		bufferCr := cairo.Create(self.buffer)
		self.renderBase(bufferCr)
		self.renderData(bufferCr)
	}
	// TODO: only copy visible area
	cr.SetSourceSurface(self.buffer, 0, 0)
	cr.Rectangle(0, 0, float64(width), float64(height))
	cr.Fill()

	if self.activeEvent != nil {
		self.showTip(cr, self.activeEvent, self.config.Render.Start, self.config.Render.Scale)
	}
}

func (self *ContextViewer) renderBase(cr *cairo.Context) {
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
	height := float64(HEADER_HEIGHT + len(self.data.Threads)*self.config.Render.Depth*BLOCK_HEIGHT)

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
		y := float64(HEADER_HEIGHT + self.config.Render.Depth*BLOCK_HEIGHT*n)
		line(0, y, width, y)

		cr.SetSourceRGB(0.4, 0.4, 0.4)
		cr.MoveTo(3.0, float64(HEADER_HEIGHT+self.config.Render.Depth*BLOCK_HEIGHT*(n+1)-5))
		cr.ShowText(self.data.Threads[n])
	}
}

func (self *ContextViewer) renderData(cr *cairo.Context) {
	cr.SetLineWidth(1.0)
	cr.SelectFontFace("sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(10)

	_rs := self.config.Render.Start
	_rc := self.config.Render.Cutoff
	_sc := self.config.Render.Scale

	shown := 0
	for _, event := range self.data.Data {
		switch {
		case event.StartType == "START":
			if (event.EndTime-event.StartTime)*1000 < _rc {
				continue
			}
			if event.Depth >= self.config.Render.Depth {
				continue
			}
			shown += 1
			if common.DEMO && shown == 500 {
				self.showError("Demo Limit", "The evaluation build is limited to showing 500 events at a time, so rendering has stopped")
				return
			}
			self.showEvent(cr, &event, _rs, _sc)

		case event.StartType == "BMARK":
			if self.config.Render.Bookmarks {
				self.showBookmark(cr, &event, _rs, _sc)
			}

		case event.StartType == "LOCKW" || event.StartType == "LOCKA":
			self.showLock(cr, &event, _rs, _sc)
		}
	}
}

func (self *ContextViewer) showEvent(cr *cairo.Context, evt *event.Event, offset_time, scale_factor float64) {
	ok := evt.EndType == "ENDOK"

	start_px := (evt.StartTime - offset_time) * scale_factor
	length_px := evt.Length() * scale_factor
	depth_px := float64(HEADER_HEIGHT + (evt.ThreadIndex * (self.config.Render.Depth * BLOCK_HEIGHT)) + (evt.Depth * BLOCK_HEIGHT))

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
	cr.ShowText(evt.Text())
	cr.Restore()
}

func (self *ContextViewer) showTip(cr *cairo.Context, evt *event.Event, offset_time, scale_factor float64) {
	cr.SetLineWidth(1.0)
	cr.SelectFontFace("sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(10)

	start_px := math.Max(0, (evt.StartTime-offset_time)*scale_factor)
	length_px := 200.0 // evt.Length() * scale_factor
	depth_px := float64(HEADER_HEIGHT + (evt.ThreadIndex * (self.config.Render.Depth * BLOCK_HEIGHT)) + (evt.Depth * BLOCK_HEIGHT))

	cr.SetSourceRGB(1.0, 1.0, 0.65)
	cr.Rectangle(math.Floor(start_px)+0.5, depth_px+0.5+BLOCK_HEIGHT, math.Floor(length_px), BLOCK_HEIGHT*2)
	cr.Fill()

	cr.SetSourceRGB(0.65, 0.65, 0.5)
	cr.Rectangle(math.Floor(start_px)+0.5, depth_px+0.5+BLOCK_HEIGHT, math.Floor(length_px), BLOCK_HEIGHT*2)
	cr.Stroke()

	cr.Save()
	cr.Rectangle(math.Floor(start_px)+0.5, depth_px+0.5+BLOCK_HEIGHT, math.Max(0, math.Floor(length_px)-5), BLOCK_HEIGHT*2)
	cr.Clip()
	cr.SetSourceRGB(0.2, 0.2, 0.2)
	cr.MoveTo(start_px+5, depth_px+BLOCK_HEIGHT*0.70+BLOCK_HEIGHT)
	cr.ShowText(evt.Tip(offset_time))
	cr.MoveTo(start_px+5, depth_px+BLOCK_HEIGHT*0.70+BLOCK_HEIGHT*2)
	cr.ShowText(evt.Text())
	cr.Restore()
}

func (self *ContextViewer) showLock(cr *cairo.Context, evt *event.Event, offset_time, scale_factor float64) {
	start_px := (evt.StartTime - offset_time) * scale_factor
	length_px := evt.Length() * scale_factor

	if evt.StartType == "LOCKW" {
		cr.SetSourceRGB(1.0, 0.85, 0.85)
	} else {
		cr.SetSourceRGB(0.85, 0.85, 1.0)
	}
	cr.Rectangle(
		start_px, float64(HEADER_HEIGHT+evt.ThreadIndex*self.config.Render.Depth*BLOCK_HEIGHT),
		length_px, float64(self.config.Render.Depth*BLOCK_HEIGHT),
	)
	cr.Fill()

	cr.SetSourceRGB(0.5, 0.5, 0.5)
	cr.Save()
	cr.Rectangle(
		start_px, float64(HEADER_HEIGHT+evt.ThreadIndex*self.config.Render.Depth*BLOCK_HEIGHT),
		length_px-5, float64(self.config.Render.Depth*BLOCK_HEIGHT),
	)
	cr.Clip()
	cr.MoveTo(start_px+5, float64(HEADER_HEIGHT+(evt.ThreadIndex+1)*self.config.Render.Depth*BLOCK_HEIGHT)-5)
	cr.ShowText(evt.Text())
	cr.Restore()
}

func (self *ContextViewer) showBookmark(cr *cairo.Context, evt *event.Event, offset_time, scale_factor float64) {
	start_px := math.Floor((evt.StartTime - offset_time) * scale_factor)
	height := float64(HEADER_HEIGHT + len(self.data.Threads)*self.config.Render.Depth*BLOCK_HEIGHT)

	cr.SetSourceRGB(1.0, 0.5, 0.0)
	cr.MoveTo(start_px+0.5, HEADER_HEIGHT)
	cr.LineTo(start_px+0.5, height)
	cr.Stroke()

	cr.MoveTo(start_px+5, height-5)
	cr.ShowText(evt.Text())
}
