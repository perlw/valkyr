package carving

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/go-ini/ini"
	"github.com/pkg/errors"
)

type DecodeFunc func(string, *ini.Section) error
type HandleFunc func(net.Conn)

type handler struct {
	Kind   string
	Decode DecodeFunc
	Handle HandleFunc
}

var handlers map[int]handler

func init() {
	handlers = make(map[int]handler)
}

func LoadRules(filename string) error {
	cfg, err := ini.Load(filename)
	if err != nil {
		return errors.Wrap(err, "could not read config")
	}
	cfg.BlockMode = false

	for _, section := range cfg.Sections() {
		spec := section.Name()
		if spec == "DEFAULT" {
			continue
		}

		name, kind := (func() (string, string) {
			parts := strings.Split(spec, "|")
			if len(parts) > 1 {
				return parts[0], parts[1]
			}
			return parts[0], ""
		})()

		found := false
		for _, handler := range handlers {
			if handler.Kind == kind {
				found = true
				err := handler.Decode(name, section)
				if err != nil {
					return errors.Wrap(err, "failed reading config for "+spec)
				}
			}
		}
		if !found {
			return errors.New("no handler for " + kind)
		}
	}

	return nil
}

func listen(port int, handler handler) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to start proxy"))
	}

	for {
		client, _ := ln.Accept()
		handler.Handle(client)
	}
}

func Serve() {
	for port, handler := range handlers {
		go listen(port, handler)
	}

	var forever chan int
	<-forever
}

func Register(port int, kind string, decode DecodeFunc, handle HandleFunc) error {
	if _, ok := handlers[port]; ok {
		return errors.New("port is already taken")
	}
	handlers[port] = handler{
		Kind:   kind,
		Decode: decode,
		Handle: handle,
	}
	log.Printf("handler: %s -> %d\n", kind, port)
	return nil
}
