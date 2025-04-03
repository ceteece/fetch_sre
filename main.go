package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
    "sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

type Endpoint struct {
	Name    string            `yaml:"name"`
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
}

type DomainStats struct {
	Success atomic.Uint64
	Total   atomic.Uint64
}

var stats = make(map[string]*DomainStats)

func checkHealth(endpoint Endpoint) {
	var client = &http.Client{
        Timeout: 500 * time.Millisecond,
    }

	bodyBytes, err := json.Marshal(endpoint)
	if err != nil {
		return
	}
	reqBody := bytes.NewReader(bodyBytes)

	req, err := http.NewRequest(endpoint.Method, endpoint.URL, reqBody)
	if err != nil {
		log.Println("Error creating request:", err)
		return
	}

	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)

	domain := extractDomain(endpoint.URL)
	stats[domain].Total.Add(1)
	if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		stats[domain].Success.Add(1)
	}
}

// TODO: this won't ignore port numbers, add test and fix
func extractDomain(url string) string {
	urlSplit := strings.Split(url, "//")
	domain := strings.Split(urlSplit[len(urlSplit)-1], "/")[0]
	return domain
}

func checkEndpoints(endpoints []Endpoint) {
	for _, endpoint := range endpoints {
		checkHealth(endpoint)
	}
}

func monitorEndpoints(endpoints []Endpoint) {
	for _, endpoint := range endpoints {
		domain := extractDomain(endpoint.URL)
		if stats[domain] == nil {
			stats[domain] = &DomainStats{}
		}
	}

    // TODO: do we want to reset domain stats after each iteration, or get the cumulative results from all iterations?
	for {
        // TODO: need to send requests in parallel, rather than serially, otherwise we'll exceed 15 second period if we have > 30 requests
        checkEndpoints(endpoints)
		logResults()

        // TODO: this adds 15s on top of the time it takes to check all of the endpoints, which isn't what we want
        //  will want to use match to ensure we always move on to next iteration after exactly 15s
        //    might need a "stop" channel to kill any outstanding checks?
		time.Sleep(15 * time.Second)
	}
}

func logResults() {
	for domain, stat := range stats {
		percentage := int(math.Round(100 * float64(stat.Success.Load()) / float64(stat.Total.Load())))
		fmt.Printf("%s has %d%% availability\n", domain, percentage)
	}
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <config_file>")
	}

	filePath := os.Args[1]
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatal("Error reading file:", err)
	}

    // TODO: make sure this gets parsed properly, including using defaults when needed
	var endpoints []Endpoint
	if err := yaml.Unmarshal(data, &endpoints); err != nil {
		log.Fatal("Error parsing YAML:", err)
	}

	monitorEndpoints(endpoints)
}
