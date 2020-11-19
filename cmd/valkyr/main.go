package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/perlw/valkyr/internal/httpproxy"
)

func loadConfig(configFile string, config interface{}) error {
	_, err := os.Stat(configFile)
	if os.IsNotExist(err) {
		return err
	}

	file, err := os.Open(configFile)
	if err != nil {
		return err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(config)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	var configFile string
	var local bool
	flag.StringVar(
		&configFile, "config-file", "/etc/valkyr.json",
		"sets the path to the config file",
	)
	flag.BoolVar(&local, "local", false, "runs valkyr locally for development")
	flag.Parse()

	logger := log.New(os.Stdout, "valkyr: ", log.LstdFlags)
	var config struct {
		AllowedHosts []string `json:"allowed_hosts"`
		Rules        []struct {
			Name        string `json:"name"`
			Match       string `json:"match"`
			Destination int    `json:"destination_port"`
		} `json:"rules"`
	}
	if err := loadConfig(configFile, &config); err != nil {
		logger.Printf(fmt.Errorf("could not load config: %w", err).Error())
		os.Exit(1)
	}

	proxyOptions := []httpproxy.ProxyOption{
		httpproxy.WithLogger(logger),
		httpproxy.WithAllowedHosts(config.AllowedHosts),
		httpproxy.WithErrorServerHeader([]string{"valkyr"}),
		httpproxy.WithErrorBody(
			[]byte(
				`the valkyr stares back at you blankly before stating;&nbsp;
				"back to Hel with you"`,
			),
		),
	}
	if !local {
		proxyOptions = append(proxyOptions, httpproxy.WithHTTPSRedirect())
	}
	proxy := httpproxy.NewProxy(proxyOptions...)
	for _, rule := range config.Rules {
		proxy.AddRule(rule.Name, rule.Match, rule.Destination)
	}

	logger.Printf("up and running")
	names := []string{}
	for _, rule := range config.Rules {
		names = append(names, rule.Name)
	}
	log.Printf("allowed hosts: %s", strings.Join(config.AllowedHosts, ", "))
	log.Printf("registered rules: %s", strings.Join(names, ", "))

	proxy.ListenAndServe()
}
