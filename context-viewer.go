package main

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	// gtk
	"./viewer/gui"
	"github.com/conformal/gotk3/gtk"
)

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

	cv := gui.ContextViewer{}
	cv.Init(filename, gui.Geometry{w, h})

	gtk.Main()
}
