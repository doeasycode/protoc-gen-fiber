package main

import (
	"flag"
	"fmt"
	"github.com/doeasycode/protoc-gen-fiber/generator/tool/gen"
	"github.com/doeasycode/protoc-gen-fiber/generator/tool/generator"
	"os"

	bmgen "github.com/doeasycode/protoc-gen-fiber/generator/bm"
)

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println(generator.Version)
		os.Exit(0)
	}

	g := bmgen.BmGenerator()
	gen.Main(g)
}
