package main

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"log"
	// gtk
	"./viewer/gui"
	"github.com/conformal/gotk3/gtk"
)

func main() {
	// add ./ to path so context-compiler can be found
	path := os.Getenv("PATH")
	newPath := filepath.Dir(os.Args[0]) + ":" + path
	err := os.Setenv("PATH", newPath)
	if err != nil {
		log.Printf("Failed to set PATH; may not be able to find context-compiler: %s", err)
	}

	var geometry = flag.String("g", "800x600", "Set window geometry")
	flag.Parse()

	var w, h int
	var e1, e2 error
	if geometry != nil {
		parts := strings.SplitN(*geometry, "x", 2)
		w, e1 = strconv.Atoi(parts[0])
		h, e2 = strconv.Atoi(parts[1])
		if e1 != nil || e2 != nil {
			log.Fatalf("Failed to parse geometry '%s', should be WxH, eg 640x480", *geometry)
		}
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
