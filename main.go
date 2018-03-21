package main

import (
	"log"

	"github.com/go-ini/ini"
	"github.com/pkg/errors"
)

func main() {
	cfg, err := ini.Load("runestone.ini")
	if err != nil {
		return errors.Wrap(err, "could not read config")
	}
	cfg.BlockMode = false

	for _, section := range cfg.Sections() {
		name := section.Name()
		if name == "DEFAULT" {
			continue
		}

		log.Println(name)
	}
}
