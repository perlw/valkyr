package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
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
	Proxy       *httputil.ReverseProxy
}

type ProxyTransport struct {
	http.RoundTripper
}

func (t *ProxyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.RoundTripper.RoundTrip(r)
	if err != nil {
		log.Printf("├%s", errors.Wrap(err, "error when talking to service"))
		resp = &http.Response{
			Status:     "500 INTERNAL SERVER ERROR",
			StatusCode: 500,
			Proto:      r.Proto,
			ProtoMajor: r.ProtoMajor,
			ProtoMinor: r.ProtoMinor,
			Header: http.Header{
				"Server": []string{"runestone"},
			},
			Body:             ioutil.NopCloser(bytes.NewBuffer([]byte("the runestone stares back at you blankly"))),
			ContentLength:    0,
			TransferEncoding: r.TransferEncoding,
			Close:            true,
			Uncompressed:     false,
			Trailer:          http.Header{},
			Request:          nil,
			TLS:              r.TLS,
		}
	}

	log.Printf("├%s", resp.Status)

	return resp, nil
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

	pathParts := []string{"/"}
	if r.URL.Path != "/" {
		pathParts = strings.Split(r.URL.Path, "/")
		pathParts[0] = "/"
	}

	longest := -1
	var matched Rule
	for _, rule := range h.Rules {
		ruleHost := strings.Split(rule.Match, "/")
		if len(ruleHost) == 0 {
			ruleHost = []string{"/"}
		} else {
			ruleHost[0] = "/"
		}
		if len(ruleHost) <= len(pathParts) {
			for t := range ruleHost {
				if ruleHost[t] != pathParts[t] {
					break
				} else {
					if t > longest {
						longest = t
						matched = rule
					}
				}
			}
		}
	}

	if longest > -1 {
		path := "/" + strings.Join(strings.Split(r.URL.Path, "/")[longest+1:], "/")
		r.URL.Path = path
		remoteUrl, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d/", matched.Destination))
		if err != nil {
			log.Println(errors.Wrap(err, "├could not parse target url"))
			return
		}

		log.Printf("├proxying to %s[%s]", matched.Name, remoteUrl)
		matched.Proxy.ServeHTTP(w, r)
		return
	}

	http.Error(w, "404 not found", http.StatusNotFound)
}

func main() {
	log.Println("┌starting up")
	handler := ProxyHandler{
		Rules: make([]Rule, 0, 10),
	}
	cfg, err := ini.Load("runestone.ini")
	if err != nil {
		log.Fatalln(errors.Wrap(err, "├could not read config"))
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
			log.Fatal(errors.Wrap(err, "├failed to map rule config"))
		}
		remoteUrl, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d/", rule.Destination))
		if err != nil {
			log.Println(errors.Wrap(err, "├could not parse target url"))
			return
		}
		rule.Proxy = httputil.NewSingleHostReverseProxy(remoteUrl)
		rule.Proxy.Transport = &ProxyTransport{http.DefaultTransport}

		handler.Rules = append(handler.Rules, rule)
	}

	names := []string{}
	for _, rule := range handler.Rules {
		names = append(names, rule.Name)
	}
	log.Println("├registered rules:", strings.Join(names, ", "))

	go (func() {
		server := &http.Server{
			Addr:           ":8000",
			Handler:        handler,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		err = server.ListenAndServe()
		if err != nil {
			log.Fatal("└could not start server,", err)
		}
	})()
	go (func() {
		server := &http.Server{
			Addr:           ":8443",
			Handler:        handler,
			TLSConfig:      &tls.Config{},
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		err = server.ListenAndServeTLS("/etc/letsencrypt/live/perlw.se/fullchain.pem", "/etc/letsencrypt/live/perlw.se/privkey.pem")
		if err != nil {
			log.Fatal("└could not start tls server,", err)
		}
	})()

	log.Println("└alive")
	var forever chan int
	<-forever
}
