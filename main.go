package main

import (
	"log"

	"github.com/go-ini/ini"
	"github.com/pkg/errors"
)

type Carving struct {
	Name      string
	Match     string `ini:"Match"`
	EntryPort int    `ini:"Port"`
	ExitPort  int    `ini:"Destination"`
}

func main() {
	cfg, err := ini.Load("runestone.ini")
	if err != nil {
		log.Fatal("Could not read config...")
	}
	cfg.BlockMode = false

	carvings := make([]Carving, 0, 10)
	for _, section := range cfg.Sections() {
		if section.Name() == "DEFAULT" {
			continue
		}

		carving := Carving{}
		if err := section.MapTo(&carving); err != nil {
			log.Fatal(errors.Wrap(err, "Failed reading section"))
		}
		carving.Name = section.Name()

		log.Println(carving)
	}

	if len(carvings) == 0 {
		log.Fatal("Nothing to done, exitting")
	}
}
