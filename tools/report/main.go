package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/MorenoLand/Moreno.AgeofZombies/engine/formats"
)

func main() {
	reference := flag.String("reference", "", "content directory")
	levelID := flag.String("level", "World0Level0", "level identifier")
	flag.Parse()
	if *reference == "" {
		log.Fatal("--reference is required")
	}
	catalog, err := formats.ParseLevelCatalog(*reference)
	if err != nil {
		log.Fatal(err)
	}
	var selected formats.LevelInfo
	for _, item := range catalog {
		if item.ID == *levelID {
			selected = item
			break
		}
	}
	if selected.ID == "" {
		log.Fatalf("level %s not found", *levelID)
	}
	level, err := formats.ParseLevel(*reference, selected)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %dx%d tileset=%s props=%d flags=%v\n", selected.ID, level.Width, level.Height, level.Tileset, len(level.Props), selected.Flags)
	for _, kind := range formats.LayerKinds {
		values := level.Layers[kind]
		nonEmpty := 0
		for _, value := range values {
			if value != ^uint32(0) {
				nonEmpty++
			}
		}
		fmt.Printf("layer %s: %d values, %d non-empty\n", kind, len(values), nonEmpty)
	}
}
