package main

import (
	//"fmt"
	//	"io"
	"log"
	//"strings"
	//"net"

	"github.com/pkg/errors"

	"github.com/perlw/runestone/carving"
	_ "github.com/perlw/runestone/carving/http"
	_ "github.com/perlw/runestone/carving/telnet"
)

func main() {
	err := carving.LoadRules("runestone.ini")
	if err != nil {
		log.Fatal(errors.Wrap(err, "could not load rules"))
	}

	carving.Serve()
}
