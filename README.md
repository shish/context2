Context 2
=========

What does it do?
----------------
Displays flame charts (a chart of stack trace vs time),
with a bunch of features for easy parsing and browsing.

![Screenshot](http://code.shishnet.org/context/context2-github-readme.png)

For a user-level guide, see http://code.shishnet.org/context/

For generating data that this viewer views, see http://github.com/shish/context-apis

Currently the viewer is GPLv3 (with the aim of having one viewer app which stands
alone) and client libraries are MIT (with the aim of them multiplying far and wide
and being embedded all over the place). If initial response to the open source
release is overwhelming sensible reasons that those are bad choices they could
be changed (though I'd only ever change to something OSI-approved).


The general idea
----------------
Apps, using one of a variety of methods (annotations, function calls, runtime profiler
hooks) write out a plain text log file of what they're doing:

```
# Timestamp      / Hostname / PID / Thread ID / Action / Function  / Comment
1417914023.671233  orchid     864   3158180608  START    top-level   Connecting to DB
1417914023.671564  orchid     864   3158180608  ENDOK    top-level 
1417914023.671589  orchid     864   3158180608  START    top-level   Loading themelets
1417914023.672227  orchid     864   2680029952  START    send_event  PostListBuildingEvent
1417914023.672259  orchid     864   2680029952  START    send_event  Upload
1417914023.672280  orchid     864   2680029952  ENDOK    send_event 
1417914023.672296  orchid     864   2680029952  START    send_event  RSS_Comments
1417914023.672320  orchid     864   2680029952  ENDOK    send_event 
1417914023.672335  orchid     864   2680029952  START    send_event  RSS_Images
```

`context-compiler` then turns this into an sqlite database for more
efficient random access and rendering. The database format is slightly
different (the main table lists whole events with `start_timestamp`
and `end_timestamp` columns, rather than individual actions). It also
contains a bunch of useful metadata like overall start and end times,
and the pre-compiled activity summary (so the navigation bar at the
bottom of the viewer can be rendered instantly).

`context-viewer` then reads this database, using SQL to search for
events in the given time window, grouping them by thread and rendering
them as a series of blocks.

Without too much difficulty it should also be possible to have the
viewer provide more advanced statistics - you can use SQL queries to
find out things like which functions get called most, or have it
display a list of events which took more than twice the average
amount of time for their type.

Note in particular that the "comment" field can contain variable data,
so rather than just "average search took 5s", you can see "search for
'foo' took 3s", "search for 'bar baz qux' took 20s".

Building
--------
The codebase should probably be more idiomatically Go-like to be built
from a single command; Context2 is a pretty much 1:1 translation of the
Python codebase though, so I apologise for some things being awkward
(patches for idiomatic Go-ness welcome :) ). Also if anyone wants a blog
entry written comparing the Python and Go code, that could probably be
arranged.

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
(Patches exceedingly welcome)
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
