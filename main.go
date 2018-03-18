package main

import (
	"fmt"
	"log"
	"net"

	"github.com/go-ini/ini"
	"github.com/pkg/errors"
)

type Carving struct {
	Name      string
	Match     string `ini:"Match"`
	EntryPort int    `ini:"Port"`
	ExitPort  int    `ini:"Destination"`
}

func proxy(port int, carvings []*Carving) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to start proxy"))
	}

	for {
		conn, _ := ln.Accept()
		conn.Close()
	}
}

func main() {
	cfg, err := ini.Load("runestone.ini")
	if err != nil {
		log.Fatal("Could not read config...")
	}
	cfg.BlockMode = false

	carvings := make([]Carving, 0, 10)
	for _, section := range cfg.Sections() {
		name := section.Name()
		if name == "DEFAULT" {
			continue
		}

		carving := Carving{}
		if err := section.MapTo(&carving); err != nil {
			log.Fatal(errors.Wrap(err, "Failed reading section"))
		}
		carving.Name = name

		carvings = append(carvings, carving)
	}

	if len(carvings) == 0 {
		log.Fatal("Nothing to done, exitting")
	}

	portMap := make(map[int][]*Carving)
	for idx, carving := range carvings {
		if _, ok := portMap[carving.EntryPort]; !ok {
			portMap[carving.EntryPort] = make([]*Carving, 0, 10)
		}
		portMap[carving.EntryPort] = append(portMap[carving.EntryPort], &carvings[idx])
	}

	log.Println("Mappings:")
	for port, carvings := range portMap {
		log.Printf("%d:\n", port)
		for _, carving := range carvings {
			log.Printf("\t%s -> %d\n", carving.Match, carving.ExitPort)
		}
	}

	for port, carvings := range portMap {
		go proxy(port, carvings)
	}

	var forever chan int
	<-forever
}
