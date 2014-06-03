package viewer

import (
	"code.google.com/p/gcfg"
	"log"
)

type Config struct {
	Render struct {
		Start    float64
		Length   float64 `gcfg:"render_len"`
		Scale    float64 `gcfg:"scale"`
		MaxDepth int     `gcfg:"max_depth"`
		Cutoff   float64 `gcfg:"render_cutoff"`
		Coalesce float64 `gcfg:"coalesce_threshold"`
		Bookmarks bool
	}
	Gui struct {
		RenderAuto bool   `gcfg:"render_auto"`
		LastLogDir string `gcfg:"last_log_dir"`
	}
	Bookmarks struct {
		Absolute bool
		Format   string
	}
}

func (self *Config) Load(configFile string) {
	err := gcfg.ReadFileInto(&self, configFile)
	if err != nil {
		log.Printf("Error loading settings from %s: %s\n", configFile, err)
	}
}

func (self *Config) Save(configFile string) {
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
