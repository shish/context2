package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"io/ioutil"
	"path/filepath"
)

func b64(filename string) string {
	name := filename
	name = strings.Replace(name, "\\", "/", -1);
	fmt.Printf("Adding %s\n", name);
	name = strings.Split(name, "/")[1];
	name = strings.Split(name, ".")[0];
	name = strings.Replace(name, "-", "_", -1);
	name = strings.ToUpper(name);
	data, _ := ioutil.ReadFile(filename);
//	for b := range(data) {
//		.join([(fmt.Sprintf("\\\\x%02X", ord(x))) for x in file(sys.argv[1]).read()]);
//	}
	return fmt.Sprintf("\t%s = %#v\n", name, string(data));
}

func main() {
	fp, _ := os.OpenFile("viewer/gui/bindata.go", os.O_WRONLY|os.O_TRUNC, 0666);
	fmt.Fprintf(fp, "package gui\n");
	fmt.Fprintf(fp, "const (\n");
	names, _ := filepath.Glob("data/*.png")
	for _, fn := range(names) {
		tmp := fmt.Sprintf("%s.dat", fn);
		exec.Command("gdk-pixbuf-pixdata", fn, tmp);
		fmt.Fprintf(fp, "%s", b64(tmp));
		os.Remove(tmp);
	}
	fmt.Fprintf(fp, ")\n");

	fp, _ = os.OpenFile("common/bindata.go", os.O_WRONLY|os.O_TRUNC, 0666);
	fmt.Fprintf(fp, "package common\n");
	fmt.Fprintf(fp, "const (\n");
	names, _ = filepath.Glob("data/*.txt")
	for _, fn := range(names) {
		fmt.Fprintf(fp, "%s", b64(fn));
	}
	fmt.Fprintf(fp, ")\n");
	
	exec.Command("go", "build", "context-compiler.go");
	exec.Command("go", "build", "context-viewer.go");
}
