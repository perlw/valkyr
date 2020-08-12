package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"

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

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Fatal(fmt.Errorf("could not set up fsnotify: %w", err).Error())
	}
	defer watcher.Close()

	var config struct {
		AllowedHosts []string `json:"allowed_hosts"`
		Rules        []struct {
			Name        string `json:"name"`
			Match       string `json:"match"`
			Destination int    `json:"destination_port"`
		} `json:"rules"`
	}
	loadConfig := func() error {
		file, err := os.Open(configFile)
		if err != nil {
			return fmt.Errorf("could not read config file: %w", err)
		}
		defer file.Close()

		err = json.NewDecoder(file).Decode(&config)
		if err != nil {
			return fmt.Errorf("could not decode config file: %w", err)
		}

		return nil
	}

	_, err = os.Stat(configFile)
	if os.IsNotExist(err) {
		logger.Printf("warning, no config file present")
	} else {
		err := loadConfig()
		if err != nil {
			logger.Fatal(fmt.Errorf("could not load config: %w", err).Error())
		}
		err = watcher.Add(configFile)
		if err != nil {
			logger.Fatal(fmt.Errorf("could watch file: %w", err).Error())
		}
	}

	proxy := httpproxy.NewProxy(
		httpproxy.WithLogger(logger),
		httpproxy.WithAllowedHosts(config.AllowedHosts),
		httpproxy.WithErrorServerHeader([]string{"valkyr"}),
		httpproxy.WithErrorBody(
			[]byte(
				`the valkyr stares back at you blankly before stating;&nbsp;
				"back to Hel with you"`,
			),
		),
	)
	for _, rule := range config.Rules {
		proxy.AddRule(rule.Name, rule.Match, rule.Destination)
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					logger.Println("config file modified, reloading", event.Name)
					err := loadConfig()
					if err != nil {
						logger.Println(fmt.Errorf("could not reload config: %w", err).Error())
					}
					proxy.SetAllowedHosts(config.AllowedHosts)
					proxy.ClearRules()
					for _, rule := range config.Rules {
						proxy.AddRule(rule.Name, rule.Match, rule.Destination)
					}
					names := []string{}
					for _, rule := range config.Rules {
						names = append(names, rule.Name)
					}
					log.Printf("allowed hosts: %s", strings.Join(config.AllowedHosts, ", "))
					log.Printf("registered rules: %s", strings.Join(names, ", "))
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	logger.Printf("up and running")
	names := []string{}
	for _, rule := range config.Rules {
		names = append(names, rule.Name)
	}
	log.Printf("allowed hosts: %s", strings.Join(config.AllowedHosts, ", "))
	log.Printf("registered rules: %s", strings.Join(names, ", "))

	proxy.ListenAndServe()
}
