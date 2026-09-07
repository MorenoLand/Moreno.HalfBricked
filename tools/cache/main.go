package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/MorenoLand/Moreno.AgeofZombies/engine/content"
)

func main() {
	reference := flag.String("reference", "", "extracted APK directory")
	output := flag.String("out", ".aoz-cache", "generated cache directory")
	flag.Parse()
	if *reference == "" {
		log.Fatal("--reference is required")
	}
	if err := content.Import(*reference, *output); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("imported cache to %s\n", *output)
}
