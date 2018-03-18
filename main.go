package main

import (
	"fmt"
	"io"
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

func proxyConnection(server, client net.Conn) {
	serverClosed := make(chan int, 1)
	clientClosed := make(chan int, 1)

	go agent(server, client, clientClosed)
	go agent(client, server, serverClosed)

	var waitFor chan int
	select {
	case <-clientClosed:
		server.Close()
		waitFor = serverClosed
	case <-serverClosed:
		client.Close()
		waitFor = clientClosed
	}

	<-waitFor
}

func agent(dest, src net.Conn, closed chan int) {
	io.Copy(dest, src)
	src.Close()
	closed <- 1
}

func proxy(port int, carvings []*Carving) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to start proxy"))
	}

	for {
		client, _ := ln.Accept()
		server, err := net.Dial("tcp", "localhost:23")
		if err != nil {
			log.Printf("could not open connection to destination, %d -> %d, %s\n", 3000, 23, "game")
			server.Close()
			client.Close()
			continue
		}

		go proxyConnection(server, client)
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
