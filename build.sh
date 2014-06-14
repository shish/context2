#!/bin/bash

cat > build.py <<EOD
import sys, base64
name = sys.argv[1].split('/')[1].split('.')[0].replace('-', '_').upper()
data = "".join([("\\\\x%02X" % ord(x)) for x in file(sys.argv[1]).read()])
print '\t%s = "%s"' % (name, data)
EOD

OUT=viewer/bindata.go
echo package viewer > $OUT
echo "const (" >> $OUT
for fn in data/*.svg ; do
	gdk-pixbuf-pixdata ${fn} ${fn}.dat
	python build.py ${fn}.dat >> $OUT
	rm -f ${fn}.dat
done
echo ")" >> $OUT

OUT=common/bindata.go
echo package common > $OUT
echo "const (" >> $OUT
for fn in data/*.txt ; do
	python build.py ${fn} >> $OUT
done
echo ")" >> $OUT

rm -f build.py

go build context-compiler.go
go build context-viewer.go
