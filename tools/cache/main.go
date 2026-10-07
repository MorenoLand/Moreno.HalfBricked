//go:build !js

package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
)

func main() {
	reference := flag.String("reference", "", "content directory")
	output := flag.String("out", ".aoz-cache", "generated cache directory")
	flag.Parse()
	if *reference == "" {
		log.Fatal("--reference is required")
	}
	var err error
	if strings.EqualFold(filepath.Ext(*reference), ".apk") {
		_, err = content.PrepareAPK(*reference, *output)
	} else {
		err = content.Import(*reference, *output)
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("imported cache to %s\n", *output)
}
