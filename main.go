package main

import (
	"flag"
	"log"
	"os"
)

var configFile = flag.String("c", "config.yaml", "Path to the configuration file, supports YAML and JSON.")

func init() {
	flag.Parse()
}

func main() {
	rawConfig := new(RawConfig)
	config := new(Config)

	if err := rawConfig.ParseRawConfig(*configFile); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	if err := config.GenerateConfig(rawConfig); err != nil {
		log.Fatal(err)
		os.Exit(2)
	}

	if err := config.SetNumCPU(); err != nil {
		log.Fatal(err)
		os.Exit(3)
	}

	for err := range config.CrawlMaxPage() {
		log.Fatal(err)
		os.Exit(4)
	}

	if err := config.GenerateCrawlList(); err != nil {
		log.Fatal(err)
		os.Exit(5)
	}

	maxCap := config.Customize.MaxCapacity
	rawResultChan := make(chan map[*string]int, maxCap)
	go config.Crawl(rawResultChan)
	config.FilterAndWrite(rawResultChan)
}
