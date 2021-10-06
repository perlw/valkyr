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

// WithErrorServerHeader is the Server Header to use when the communication
// between the proxy and the matched service fails in some way.
func WithErrorServerHeader(serverHeader []string) ProxyOption {
	return func(p *Proxy) {
		p.errorServerHeader = serverHeader
	}
}

// WithErrorBody is the response body to use when the communication between the
// proxy and the matched service fails in some way.
func WithErrorBody(body []byte) ProxyOption {
	return func(p *Proxy) {
		p.errorBody = body
	}
}

// WithHTTPSRedirect redirects insecure HTTP calls to HTTPS.
func WithHTTPSRedirect() ProxyOption {
	return func(p *Proxy) {
		p.httpsRedirect = true
	}
}

// Proxy is the main proxy struct.
type Proxy struct {
	logger            *log.Logger
	allowedHosts      []string
	handler           proxyHandler
	ruleMutex         sync.RWMutex
	certMan           *autocert.Manager
	errorServerHeader []string
	errorBody         []byte
	httpsRedirect     bool

	// Metrics
	VisitsPerServiceAndPath map[string]map[string]int
	StatusPerServiceAndPath map[string]map[string]map[int]int
	TotalVisits             int
	TotalErrors             int
}

// NewProxy sets up everything needed to get a running proxy.
// TODO: Make Proxy less chatty?
func NewProxy(opts ...ProxyOption) *Proxy {
	p := Proxy{
		logger:            nil,
		allowedHosts:      []string{},
		errorServerHeader: []string{"httpproxy"},
		errorBody:         []byte("error communicating with matched service"),

		VisitsPerServiceAndPath: make(map[string]map[string]int),
		StatusPerServiceAndPath: make(map[string]map[string]map[int]int),
	}

	for _, opt := range opts {
		opt(&p)
	}

	p.handler.logger = p.logger
	p.handler.ruleMutex = &p.ruleMutex
	p.handler.proxy = &p
	p.certMan = &autocert.Manager{
		Cache:      autocert.DirCache("certs"),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(p.allowedHosts...),
	}

	return &p
}

// TODO: Look into lru, this will currently fill memory extremely easily.
func (p *Proxy) incVisitMetric(service, path string) {
	if _, ok := p.VisitsPerServiceAndPath[service]; !ok {
		p.VisitsPerServiceAndPath[service] = make(map[string]int)
	}
	p.VisitsPerServiceAndPath[service][path]++
	p.TotalVisits++
}

// TODO: Look into lru, this will currently fill memory extremely easily.
func (p *Proxy) incStatusMetric(service, path string, status int) {
	if _, ok := p.StatusPerServiceAndPath[service]; !ok {
		p.StatusPerServiceAndPath[service] = make(map[string]map[int]int)
	}
	if _, ok := p.StatusPerServiceAndPath[service][path]; !ok {
		p.StatusPerServiceAndPath[service][path] = make(map[int]int)
	}
	p.StatusPerServiceAndPath[service][path][status]++
	if status >= 400 {
		p.TotalErrors++
	}
}

// SetAllowedHosts sets the allowed hosts.
func (p *Proxy) SetAllowedHosts(hosts []string) {
	p.allowedHosts = hosts
	p.certMan.HostPolicy = autocert.HostWhitelist(p.allowedHosts...)
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
		logger:            p.logger,
		proxy:             p,
		errorServerHeader: p.errorServerHeader,
		errorBody:         p.errorBody,
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

// ClearRules clears all rules from the proxy.
func (p *Proxy) ClearRules() {
	p.ruleMutex.Lock()
	defer p.ruleMutex.Unlock()
	p.handler.Rules = []rule{}
}

// ListenAndServe starts the proxy.
func (p *Proxy) ListenAndServe() {
	serverFailed := make(chan interface{})
	tlsServerFailed := make(chan interface{})
	go (func() {
		defer close(serverFailed)

		var handler http.Handler
		if p.httpsRedirect {
			handler = p.certMan.HTTPHandler(nil)
		} else {
			handler = p.handler
		}
		server := &http.Server{
			Addr:           ":8000",
			Handler:        handler,
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
				GetCertificate: p.certMan.GetCertificate,
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

	go (func() {
		server := &http.Server{
			Addr: ":9090",
			Handler: metricsHandler{
				proxy: p,
				rules: p.handler.Rules,
			},
			ReadTimeout:    90 * time.Second,
			WriteTimeout:   90 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		err := server.ListenAndServe()
		if err != nil {
			p.logger.Fatal("could not serve metrics,", err)
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
	logger            *log.Logger
	proxy             *Proxy
	errorServerHeader []string
	errorBody         []byte
}

func (t *proxyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.RoundTripper.RoundTrip(r)
	if err != nil {
		t.logger.Printf("error when talking to service: %s", err.Error())
		resp = &http.Response{
			Status:     "500 INTERNAL SERVER ERROR",
			StatusCode: 500,
			Proto:      r.Proto,
			ProtoMajor: r.ProtoMajor,
			ProtoMinor: r.ProtoMinor,
			Header: http.Header{
				"Server": t.errorServerHeader,
			},
			Body:             ioutil.NopCloser(bytes.NewBuffer(t.errorBody)),
			ContentLength:    0,
			TransferEncoding: r.TransferEncoding,
			Close:            true,
			Uncompressed:     false,
			Trailer:          http.Header{},
			Request:          nil,
			TLS:              r.TLS,
		}
	}

	t.proxy.incStatusMetric("valkyr", r.URL.String(), resp.StatusCode)

	return resp, nil
}

type proxyHandler struct {
	logger    *log.Logger
	ruleMutex *sync.RWMutex
	proxy     *Proxy
	Rules     []rule
}

// TODO: Cleanup matching logic.
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
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) == longest+1 {
			h.logger.Printf(
				"redirecting to %s:%s, missing root", matched.Name, r.URL.String()+"/",
			)
			h.proxy.incStatusMetric(matched.Name, r.URL.String(), http.StatusMovedPermanently)
			http.Redirect(w, r, r.URL.String()+"/", http.StatusMovedPermanently)
			return
		}
		path := "/" + strings.Join(parts[longest+1:], "/")
		r.URL.Path = path

		h.proxy.incVisitMetric(matched.Name, r.URL.String())
		h.logger.Printf("proxying to %s:%s", matched.Name, path)
		matched.Proxy.ServeHTTP(w, r)
		return
	}

	h.proxy.incVisitMetric("valkyr", r.URL.String())
	h.proxy.incStatusMetric("valkyr", r.URL.String(), http.StatusNotFound)
	http.Error(w, "404 NOT FOUND", http.StatusNotFound)
}

type metricsHandler struct {
	proxy *Proxy
	rules []rule
}

func (h metricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.String() == "/metrics" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("services:\n"))
		services := make([]string, 0, 10)
		services = append(services, "valkyr")
		for _, r := range h.rules {
			services = append(services, r.Name)
		}
		for _, service := range services {
			w.Write([]byte(fmt.Sprintf("\t%s\n", service)))
			for path, count := range h.proxy.VisitsPerServiceAndPath[service] {
				w.Write([]byte(fmt.Sprintf("\t\t%s: %d\n", path, count)))
				for statusCode, statusCount := range h.proxy.StatusPerServiceAndPath[service][path] {
					w.Write([]byte(fmt.Sprintf("\t\t\t%d: %d\n", statusCode, statusCount)))
				}
			}
		}
		w.Write([]byte("---\n"))
		w.Write([]byte(fmt.Sprintf("total_visits: %d\n", h.proxy.TotalVisits)))
		w.Write([]byte(fmt.Sprintf("total_errors: %d\n", h.proxy.TotalErrors)))
	}
}
