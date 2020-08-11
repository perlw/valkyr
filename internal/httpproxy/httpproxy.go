package httpproxy

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// ProxyOption is the definition for a Proxy Option func.
type ProxyOption func(p *Proxy)

// WithLogger adds a logger to the proxy.
func WithLogger(logger *log.Logger) ProxyOption {
	return func(p *Proxy) {
		p.logger = logger
	}
}

// WithAllowedHosts sets the allowed hosts.
func WithAllowedHosts(hosts []string) ProxyOption {
	return func(p *Proxy) {
		p.allowedHosts = hosts
	}
}

// Proxy is the main proxy struct.
type Proxy struct {
	logger       *log.Logger
	allowedHosts []string
	handler      proxyHandler
	ruleMutex    sync.RWMutex
}

// NewProxy sets up everything needed to get a running proxy.
func NewProxy(opts ...ProxyOption) *Proxy {
	p := Proxy{
		logger:       nil,
		allowedHosts: []string{},
	}

	for _, opt := range opts {
		opt(&p)
	}

	p.handler.logger = p.logger
	p.handler.ruleMutex = &p.ruleMutex

	return &p
}

// AddRule adds a new rule to the proxy engine.
func (p *Proxy) AddRule(name string, match string, destinationPort int) error {
	p.ruleMutex.Lock()
	defer p.ruleMutex.Unlock()

	remoteURL, err := url.Parse(
		fmt.Sprintf("http://127.0.0.1:%d/", destinationPort),
	)
	if err != nil {
		return fmt.Errorf("could not parse target url: %w", err)
	}
	reverseProxy := httputil.NewSingleHostReverseProxy(remoteURL)
	reverseProxy.Transport = &proxyTransport{
		logger: p.logger,
	}
	reverseProxy.Transport = http.DefaultTransport
	p.handler.Rules = append(p.handler.Rules, rule{
		Name:        name,
		Match:       match,
		Destination: destinationPort,
		Proxy:       reverseProxy,
	})

	return nil
}

// ListenAndServe starts the proxy.
func (p *Proxy) ListenAndServe() {
	m := &autocert.Manager{
		Cache:      autocert.DirCache("certs"),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(p.allowedHosts...),
	}
	serverFailed := make(chan interface{})
	tlsServerFailed := make(chan interface{})
	go (func() {
		defer close(serverFailed)

		server := &http.Server{
			Addr:           ":8000",
			Handler:        m.HTTPHandler(nil),
			ReadTimeout:    90 * time.Second,
			WriteTimeout:   90 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		err := server.ListenAndServe()
		if err != nil {
			p.logger.Fatal("could not start server,", err)
		}
	})()
	go (func() {
		defer close(tlsServerFailed)

		server := &http.Server{
			Addr:    ":8443",
			Handler: p.handler,
			TLSConfig: &tls.Config{
				GetCertificate: m.GetCertificate,
			},
			ReadTimeout:    90 * time.Second,
			WriteTimeout:   90 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		err := server.ListenAndServeTLS("", "")
		if err != nil {
			p.logger.Fatal("could not start tls server,", err)
		}
	})()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case <-stop:
	case <-serverFailed:
	case <-tlsServerFailed:
	}
}

type rule struct {
	Name        string
	Match       string
	Destination int
	Proxy       *httputil.ReverseProxy
}

type proxyTransport struct {
	http.RoundTripper
	logger *log.Logger
}

func (t *proxyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.RoundTripper.RoundTrip(r)
	if err != nil {
		t.logger.Println(fmt.Errorf("error when talking to service: %w", err).Error())
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

	return resp, nil
}

type proxyHandler struct {
	logger    *log.Logger
	ruleMutex *sync.RWMutex
	Rules     []rule
}

func (h proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.ruleMutex.RLock()
	defer h.ruleMutex.RUnlock()

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
		remoteURL, _ := url.Parse(
			fmt.Sprintf("http://127.0.0.1:%d/", matched.Destination),
		)

		h.logger.Printf("proxying to %s[%s]", matched.Name, remoteURL)
		matched.Proxy.ServeHTTP(w, r)
		return
	}

	http.Error(w, "404 NOT FOUND", http.StatusNotFound)
}
