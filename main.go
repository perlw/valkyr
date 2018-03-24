package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/go-ini/ini"
	"github.com/pkg/errors"
)

type Rule struct {
	Name        string
	Match       string `ini:"match"`
	Destination int    `ini:"destination"`
}

type ProxyHandler struct {
	Rules []Rule
}

func (h ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("┌%s", r.URL.Path)
	defer func() {
		log.Printf("└done in %.2fms", float64(time.Since(start))/float64(time.Millisecond))
	}()

	host := strings.Split(r.Host, ":")[0]

	for _, rule := range h.Rules {
		if rule.Match == host {
			remoteUrl, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", rule.Destination))
			if err != nil {
				log.Println(errors.Wrap(err, "├could not parse target url"))
				return
			}

			log.Printf("├proxying to %s", remoteUrl)
			proxy := httputil.NewSingleHostReverseProxy(remoteUrl)
			proxy.ServeHTTP(w, r)
			return
		}
	}

	http.Error(w, "404 not found", http.StatusNotFound)
}

func main() {
	handler := ProxyHandler{
		Rules: make([]Rule, 0, 10),
	}
	cfg, err := ini.Load("runestone.ini")
	if err != nil {
		log.Fatalln(errors.Wrap(err, "could not read config"))
	}
	cfg.BlockMode = false

	for _, section := range cfg.Sections() {
		name := section.Name()
		if name == "DEFAULT" {
			continue
		}

		rule := Rule{
			Name: name,
		}
		if err := section.MapTo(&rule); err != nil {
			log.Fatal(errors.Wrap(err, "failed to map rule config"))
		}
		handler.Rules = append(handler.Rules, rule)
	}

	names := []string{}
	for _, rule := range handler.Rules {
		names = append(names, rule.Name)
	}
	log.Println("registered rules:", strings.Join(names, ", "))

	server := &http.Server{
		Addr:           ":8000",
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Println("going up...")
	err = server.ListenAndServe()
	if err != nil {
		log.Fatal("could not start server,", err)
	}
}
