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
	"golang.org/x/crypto/acme/autocert"
)

type rule struct {
	Name        string
	Match       string `ini:"match"`
	Destination int    `ini:"destination"`
	Proxy       *httputil.ReverseProxy
}

type proxyTransport struct {
	http.RoundTripper
}

func (t *proxyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.RoundTripper.RoundTrip(r)
	if err != nil {
		log.Printf("├%s", fmt.Errorf("error when talking to service: %w", err))
		resp = &http.Response{
			Status:     "500 INTERNAL SERVER ERROR",
			StatusCode: 500,
			Proto:      r.Proto,
			ProtoMajor: r.ProtoMajor,
			ProtoMinor: r.ProtoMinor,
			Header: http.Header{
				"Server": []string{"valkyr"},
			},
			Body: ioutil.NopCloser(
				bytes.NewBuffer(
					[]byte("the valkyr stares back at you blankly before stating; \"back to Hel with you\""),
				),
			),
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

type proxyHandler struct {
	Rules []rule
}

func (h proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("┌%s", r.URL.Path)
	defer func() {
		log.Printf(
			"└done in %.2fms", float64(time.Since(start))/float64(time.Millisecond),
		)
	}()

	reqHost := strings.Split(r.Host, ":")[0]

	pathParts := []string{"/"}
	if r.URL.Path != "/" {
		pathParts = strings.Split(r.URL.Path, "/")
		pathParts[0] = "/"
	}

	longest := -1
	var matched rule
	for _, rule := range h.Rules {
		ruleHostPath := strings.Split(rule.Match, "/")
		ruleHost := ruleHostPath[0]
		ruleHostPath[0] = "/"

		if ruleHost != reqHost {
			continue
		}

		if len(ruleHostPath) <= len(pathParts) {
			for t := range ruleHostPath {
				if ruleHostPath[t] != pathParts[t] {
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
		remoteURL, err := url.Parse(
			fmt.Sprintf("http://127.0.0.1:%d/", matched.Destination),
		)
		if err != nil {
			log.Println(fmt.Errorf("├could not parse target url: %w", err))
			return
		}

		log.Printf("├proxying to %s[%s]", matched.Name, remoteURL)
		matched.Proxy.ServeHTTP(w, r)
		return
	}

	http.Error(w, "404 not found", http.StatusNotFound)
}

func main() {
	log.Println("┌starting up")
	handler := proxyHandler{
		Rules: make([]rule, 0, 10),
	}
	cfg, err := ini.Load("valkyr.ini")
	if err != nil {
		log.Fatalln(fmt.Errorf("├could not read config: %w", err))
	}
	cfg.BlockMode = false

	for _, section := range cfg.Sections() {
		name := section.Name()
		if name == "DEFAULT" {
			continue
		}

		rule := rule{
			Name: name,
		}
		if err := section.MapTo(&rule); err != nil {
			log.Fatal(fmt.Errorf("├failed to map rule config: %w", err))
		}
		remoteURL, err := url.Parse(
			fmt.Sprintf("http://127.0.0.1:%d/", rule.Destination),
		)
		if err != nil {
			log.Println(fmt.Errorf("├could not parse target url: %w", err))
			return
		}
		rule.Proxy = httputil.NewSingleHostReverseProxy(remoteURL)
		rule.Proxy.Transport = &proxyTransport{http.DefaultTransport}

		handler.Rules = append(handler.Rules, rule)
	}

	names := []string{}
	for _, rule := range handler.Rules {
		names = append(names, rule.Name)
	}
	log.Println("├registered rules:", strings.Join(names, ", "))

	m := &autocert.Manager{
		Cache:      autocert.DirCache("certs"),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist("perlw.se", "pondofsolace.se"),
	}
	go (func() {
		server := &http.Server{
			Addr:           ":8000",
			Handler:        m.HTTPHandler(nil),
			ReadTimeout:    90 * time.Second,
			WriteTimeout:   90 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		err = server.ListenAndServe()
		if err != nil {
			log.Fatal("└could not start server,", err)
		}
	})()
	go (func() {
		server := &http.Server{
			Addr:    ":8443",
			Handler: handler,
			TLSConfig: &tls.Config{
				GetCertificate: m.GetCertificate,
			},
			ReadTimeout:    90 * time.Second,
			WriteTimeout:   90 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		err = server.ListenAndServeTLS("", "")
		if err != nil {
			log.Fatal("└could not start tls server,", err)
		}
	})()

	log.Println("└alive")
	var forever chan int
	<-forever
}
