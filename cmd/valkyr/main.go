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

func main() {
	var configFile string
	flag.StringVar(
		&configFile, "config-file", "/etc/valkyr.json",
		"sets the path to the config file",
	)
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

	_, err := os.Stat(configFile)
	if os.IsNotExist(err) {
		logger.Printf("warning, no config file present")
	} else {
		file, err := os.Open(configFile)
		if err != nil {
			logger.Fatal(fmt.Errorf("could not read config file: %w", err).Error())
		}
		defer file.Close()

		err = json.NewDecoder(file).Decode(&config)
		if err != nil {
			logger.Fatal(fmt.Errorf("could not decode config file: %w", err).Error())
		}
	}

	proxy := httpproxy.NewProxy(
		httpproxy.WithLogger(logger),
		httpproxy.WithAllowedHosts(config.AllowedHosts),
	)
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
