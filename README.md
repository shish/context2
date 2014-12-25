Context 2
=========

What does it do?
----------------
Displays fire charts (a chart of stack trace vs time),
with a bunch of features for easy parsing and browsing.

For a user-level guide, see http://code.shishnet.org/context/


Building
--------
The codebase should probably be more idiomatically Go-like to be built
from a single command; Context2 is a pretty much 1:1 translation of the
Python codebase though, so I apologise for some things being awkward
(patches for idiomatic Go-ness welcome :) )

Build the static assets (ie, turning icon SVGs into byte arrays in .go
files so they can be statically linked with the main binary)
```sh
go run build-assets.go
```

Build the compiler (.ctxt (plain text log) to .cbin (sqlite database)
translator)
```sh
go get github.com/mxk/go-sqlite/sqlite3
go build context-compiler.go
```

Build the GTK front end (.cbin viewer). The process of building against a forked
version of an upstream library is terrible and I don't know how to fix it D:
```
sudo apt-get install libgtk-3-dev
go get github.com/conformal/gotk3/gtk
cd ~/.gocode/src/github.com/conformal/gotk3
git remote add shish https://github.com/shish/gotk3
git fetch --all
git checkout shish-master
cd -
go get -tags gtk_3_10 github.com/conformal/gotk3/gtk  # change to your current GTK version
go build context-viewer.go
```
