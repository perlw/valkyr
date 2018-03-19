package telnet

import (
	"fmt"
	"io"
	"log"
	"net"

	"github.com/go-ini/ini"
	"github.com/pkg/errors"

	"github.com/perlw/runestone/carving"
)

const TelnetPort = 23

type Rule struct {
	Name        string
	Destination int `ini:"Destination"`
}

func streamer(dest, src net.Conn, closed chan int) {
	io.Copy(dest, src)
	src.Close()
	closed <- 1
}

func (Rule) Proxy(client net.Conn, server net.Conn) {
	serverClosed := make(chan int, 1)
	clientClosed := make(chan int, 1)

	go streamer(server, client, clientClosed)
	go streamer(client, server, serverClosed)

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

func (r Rule) String() string {
	return fmt.Sprintf("telnet:%s->%d", r.Name, r.Destination)
}

var rules []Rule

func Decode(name string, section *ini.Section) error {
	rule := Rule{
		Name: name,
	}
	if err := section.MapTo(&rule); err != nil {
		return errors.Wrap(err, "failed to map rule")
	}
	log.Println(rule)
	rules = append(rules, rule)
	return nil
}

func Handle(client net.Conn) {

	// TODO: Set up actual BBS-like portal
	log.Println("telnet handler")
	if len(rules) > 0 {
		rule := rules[0]

		log.Printf("proxy:[%s|telnet] from %s to :%d", rule.Name, client.RemoteAddr(), rule.Destination)
		server, err := net.Dial("tcp", fmt.Sprintf(":%d", rule.Destination))
		if err != nil {
			log.Printf("failed, %d -> %d, %s\n", rule.Destination, TelnetPort, rule.Name)
			client.Close()
			return
		}

		go rule.Proxy(client, server)
	}
}

func init() {
	rules = make([]Rule, 0, 10)
	carving.Register(TelnetPort, "telnet", Decode, Handle)
}
