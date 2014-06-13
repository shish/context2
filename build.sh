#!/bin/bash

cat >common/data.go <<EOD
package common

const LICENSE = \`$(cat LICENSE.txt)\`
EOD

go build context-compiler.go
go build context-viewer.go
