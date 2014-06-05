package viewer

import (
	"encoding/json"
	"os"
	"os/user"
)

type Config struct {
	Render struct {
		Start     float64
		Length    float64
		Scale     float64
		MaxDepth  int
		Cutoff    float64
		Coalesce  float64
		Bookmarks bool
	}
	Gui struct {
		BookmarkPanel bool
		RenderAuto    bool
		LastLogDir    string
	}
	Bookmarks struct {
		Absolute bool
		Format   string
	}
}

func (self *Config) Default() {
	usr, _ := user.Current()

	self.Render.Start = 0
	self.Render.Length = 20.0
	self.Render.Scale = 50.0
	self.Render.MaxDepth = 7
	self.Render.Cutoff = 0.0
	self.Render.Coalesce = 0.0
	self.Render.Bookmarks = false

	self.Gui.BookmarkPanel = true
	self.Gui.RenderAuto = true
	self.Gui.LastLogDir = usr.HomeDir

	self.Bookmarks.Absolute = true
	self.Bookmarks.Format = "2006/01/02 15:04:05"
}

func (self *Config) Load(configFile string) (err error) {
	buf := make([]byte, 2048)
	self.Default()

	fp, err := os.Open(configFile)
	if err != nil {
		return
	}

	n, err := fp.Read(buf)
	if err != nil {
		return
	}

	err = json.Unmarshal(buf[:n], self)
	if err != nil {
		return
	}

	return nil
}

func (self *Config) Save(configFile string) (err error) {
	fp, err := os.Create(configFile)
	if err != nil {
		return
	}

	b, err := json.MarshalIndent(self, "", "    ")
	if err != nil {
		return
	}

	_, err = fp.Write(b)
	if err != nil {
		return
	}

	return
}
