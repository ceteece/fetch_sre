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
    "sync"
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

	bodyBytes, err := json.Marshal(endpoint.Body)
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

func extractDomain(url string) string {
    url, found := strings.CutPrefix(url, "https://")
    if !found {
        url = strings.TrimPrefix(url, "http://")
    }

	authority := strings.Split(url, "/")[0]
    domain := strings.Split(authority, ":")[0]

	return domain
}

func checkEndpoints(endpoints []Endpoint) {
    var wg sync.WaitGroup

	for _, endpoint := range endpoints {
        wg.Add(1)

		go func() {
            defer wg.Done()
            checkHealth(endpoint)
        }()
	}

    wg.Wait()
}

func monitorEndpoints(endpoints []Endpoint) {
	for _, endpoint := range endpoints {
		domain := extractDomain(endpoint.URL)
		if stats[domain] == nil {
			stats[domain] = &DomainStats{}
		}
	}

    next_log_time := time.Now()
	for {
        checkEndpoints(endpoints)

		time.Sleep(time.Until(next_log_time))
        logResults()

        next_log_time = next_log_time.Add(15 * time.Second)
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

	var endpoints []Endpoint
	if err := yaml.Unmarshal(data, &endpoints); err != nil {
		log.Fatal("Error parsing YAML:", err)
	}

	monitorEndpoints(endpoints)
}
