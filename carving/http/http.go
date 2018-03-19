package http

import (
	"log"
	"net"

	"github.com/go-ini/ini"
	"github.com/pkg/errors"

	"github.com/perlw/runestone/carving"
)

type Rule struct {
	Name        string
	Match       string `ini:"Match"`
	Destination int    `ini:"Destination"`
}

func (Rule) Proxy() {
}

func Decode(name string, section *ini.Section) error {
	rule := Rule{
		Name: name,
	}
	if err := section.MapTo(&rule); err != nil {
		return errors.Wrap(err, "failed to map rule")
	}
	return nil
}

func Handle(client net.Conn) {
	log.Println("http handler")
	client.Close()
}

func init() {
	carving.Register(80, "http", Decode, Handle)
}
