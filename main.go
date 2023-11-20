package main

import (
	"flag"
	"fmt"
	"os"
	"protoc-gen-fiber/generator/tool/gen"
	"protoc-gen-fiber/generator/tool/generator"

	bmgen "protoc-gen-fiber/generator/bm"
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
