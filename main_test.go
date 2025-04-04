package main
import (
    "bufio"
    "context"
    "encoding/json"
    "io"
    "net"
    "net/http"
    "net/http/httptest"
    "os/exec"
    "strings"
    "testing"
    "time"
)

func TestExtractDomain(t *testing.T) {
    tests := []struct {
        name string
        url string
        expected string
    }{
        {"basic_url", "https://somesite.com", "somesite.com"},
        {"with_path", "https://test.net/some/path", "test.net"},
        {"path_with_double_slash", "https://test.net/some//path", "test.net"},
        {"with_port", "http://somesite.com:80/path/", "somesite.com"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := extractDomain(tt.url)
            if result != tt.expected {
                t.Errorf("returned %s, expected %s", result, tt.expected)
            }
        })
    }
}

func TestCheckHealth(t *testing.T) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch r.URL.Path {
        case "/server_err":
            w.WriteHeader(http.StatusInternalServerError)
        case "/slow":
            time.Sleep(5 * time.Second)            
            w.WriteHeader(http.StatusOK)
        default:
            w.WriteHeader(http.StatusOK)
        }
    })

    server := httptest.NewServer(handler)
    defer server.Close()

    domain := extractDomain(server.URL)

    // TODO: add tests for other REST methods, with/without body and header
    tests := []struct {
        name string
        endpoint Endpoint
        success uint64
        total uint64
    }{
        {"basic_200", Endpoint{"test", server.URL, "GET", make(map[string]string), ""}, 1, 1},
        {"basic_500", Endpoint{"test", server.URL + "/server_err", "GET", make(map[string]string), ""}, 0, 1},
        {"slow", Endpoint{"test", server.URL + "/slow", "GET", make(map[string]string), ""}, 0, 1},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            stats[domain] = &DomainStats{}
            checkHealth(tt.endpoint)
            if stats[domain].Success.Load() != tt.success || stats[domain].Total.Load() != tt.total {
                t.Errorf("stats is %d/%d, expected %d/%d", stats[domain].Success.Load(), stats[domain].Total.Load(), tt.success, tt.total)
            }
        })
    }
}

func TestCheckEndpoints(t *testing.T) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(250 * time.Millisecond)
        w.WriteHeader(http.StatusOK)
    })

    server := httptest.NewServer(handler)
    defer server.Close()

    domain := extractDomain(server.URL)

    endpoints := make([]Endpoint, 0)
    for range 100 {
        endpoints = append(endpoints, Endpoint{"test", server.URL, "GET", make(map[string]string), ""})
    }

    stats[domain] = &DomainStats{}

    start := time.Now()
    checkEndpoints(endpoints)
    end := time.Now()

    runtime := end.Sub(start).Seconds()

    if runtime >= 15.0 {
        t.Errorf("took %f seconds to check all endpoints, should be <15s", runtime)
    }
 
    // confirm that all checks actually completed
    if stats[domain].Total.Load() != 100 {
        t.Errorf("only %d out of 100 health checks completed", stats[domain].Total.Load())
    }
}

func parseOutput(line string) (string, string) {
    split := strings.Split(line, " ")

    return split[0], split[2]
}

// TODO: check that there is no body
func TestGetOK(t *testing.T) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "GET" {
            t.Errorf("got HTTP method %s, expected GET", r.Method)
        }

        w.WriteHeader(http.StatusOK)
    })

    url := "127.0.0.1:35580"
    l, err := net.Listen("tcp", url)
    if err != nil {
        t.Fatalf("failed to listen on %s", url)
    }

    server := httptest.NewUnstartedServer(handler)
    server.Listener.Close()
    server.Listener = l

    server.Start()
    defer server.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 3 * time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "go", "run", "main.go", "testdata/basic.yaml")
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        t.Fatalf("failed to create stdout pipe")
    }

    go cmd.Run()

    scanner := bufio.NewScanner(stdout)
    if scanner.Scan() {
        line := scanner.Text()
        domain, availability := parseOutput(line)
        if domain != "127.0.0.1" || availability != "100%" {
            t.Errorf("got %s availability for %s, expected 100%% availability for 127.0.0.1", availability, domain)
        }
    } else {
        t.Errorf("no availability information was printed")
    }
}

func TestGetFail(t *testing.T) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "GET" {
            t.Errorf("got HTTP method %s, expected GET", r.Method)
        }

        w.WriteHeader(http.StatusInternalServerError)
    })

    url := "127.0.0.1:35580"
    l, err := net.Listen("tcp", url)
    if err != nil {
        t.Fatalf("failed to listen on %s", url)
    }

    server := httptest.NewUnstartedServer(handler)
    server.Listener.Close()
    server.Listener = l

    server.Start()
    defer server.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 3 * time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "go", "run", "main.go", "testdata/basic.yaml")
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        t.Fatalf("failed to create stdout pipe")
    }

    go cmd.Run()

    scanner := bufio.NewScanner(stdout)
    if scanner.Scan() {
        line := scanner.Text()
        domain, availability := parseOutput(line)
        if domain != "127.0.0.1" || availability != "0%" {
            t.Errorf("got %s availability for %s, expected 0%% availability for 127.0.0.1", availability, domain)
        }
    } else {
        t.Errorf("no availability information was printed")
    }
}

func TestPostOK(t *testing.T) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            t.Errorf("got HTTP method %s, expected POST", r.Method)
        }

        if r.URL.Path != "/body" {
            t.Errorf("got request for path %s, expected /body", r.URL.Path)
        }

        if r.Header.Get("content-type") != "application/json" {
            t.Errorf("got content-type %s, expected application/json", r.Header["content-type"])
        }

        body, err :=  io.ReadAll(r.Body)
        if err != nil {
            t.Errorf("failed to read request body")
        }

        var body_string string
        err = json.Unmarshal(body, &body_string)
        if err != nil {
            t.Errorf("failed to unmarshal request body")
        }

        if body_string != "{\"foo\": \"bar\"}" {
            t.Errorf("got body %s, expected {\"foo\": \"bar\"}", body_string)
        }

        w.WriteHeader(http.StatusOK)
    })

    url := "127.0.0.1:35580"
    l, err := net.Listen("tcp", url)
    if err != nil {
        t.Fatalf("failed to listen on %s", url)
    }

    server := httptest.NewUnstartedServer(handler)
    server.Listener.Close()
    server.Listener = l

    server.Start()
    defer server.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 3 * time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "go", "run", "main.go", "testdata/post.yaml")
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        t.Fatalf("failed to create stdout pipe")
    }

    go cmd.Run()

    scanner := bufio.NewScanner(stdout)
    if scanner.Scan() {
        line := scanner.Text()
        domain, availability := parseOutput(line)
        if domain != "127.0.0.1" || availability != "100%" {
            t.Errorf("got %s availability for %s, expected 100%% availability for 127.0.0.1", availability, domain)
        }
    } else {
        t.Errorf("no availability information was printed")
    }
}

func TestSlow(t *testing.T) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "GET" {
            t.Errorf("got HTTP method %s, expected GET", r.Method)
        }

        time.Sleep(750 * time.Millisecond)
        w.WriteHeader(http.StatusOK)
    })

    url := "127.0.0.1:35580"
    l, err := net.Listen("tcp", url)
    if err != nil {
        t.Fatalf("failed to listen on %s", url)
    }

    server := httptest.NewUnstartedServer(handler)
    server.Listener.Close()
    server.Listener = l

    server.Start()
    defer server.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 40 * time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "go", "run", "main.go", "testdata/basic.yaml")
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        t.Fatalf("failed to create stdout pipe")
    }

    go cmd.Run()

    i := 0
    scanner := bufio.NewScanner(stdout)
    if scanner.Scan() {
        current_time := time.Now()

        line := scanner.Text()
        domain, availability := parseOutput(line)
        if domain != "127.0.0.1" || availability != "0%" {
            t.Errorf("got %s availability for %s, expected 0%% availability for 127.0.0.1", availability, domain)
        }

        i++

        for i < 3 && scanner.Scan() {
            prev_time := current_time
            current_time = time.Now()
            time_diff := current_time.Sub(prev_time).Milliseconds()
            if time_diff > 15400 || time_diff < 14600 {
                t.Errorf("time difference between intervals was %d ms, expected time to be within 14600ms - 15400ms", time_diff)
            }

            line := scanner.Text()
            domain, availability := parseOutput(line)
            if domain != "127.0.0.1" || availability != "0%" {
                t.Errorf("got %s availability for %s, expected 0%% availability for 127.0.0.1", availability, domain)
            }

            i++
        }
    }

    if i < 3 {
        t.Errorf("got only %d lines of availability info, expected 3", i)
    }
}

func TestMultipleDomains(t *testing.T) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/bad" {
            w.WriteHeader(http.StatusInternalServerError)
        } else {
            w.WriteHeader(http.StatusOK)
        }
    })

    for _, url := range [2]string{"127.0.0.1:35580", "127.0.0.2:35581"} {
        l, err := net.Listen("tcp", url)
        if err != nil {
            t.Fatalf("failed to listen on %s", url)
        }

        server := httptest.NewUnstartedServer(handler)
        server.Listener.Close()
        server.Listener = l

        server.Start()
        defer server.Close()
    }

    ctx, cancel := context.WithTimeout(context.Background(), 3 * time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "go", "run", "main.go", "testdata/multiple_domains.yaml")
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        t.Fatalf("failed to create stdout pipe")
    }

    go cmd.Run()

    i := 0
    scanner := bufio.NewScanner(stdout)
    for i < 2 && scanner.Scan() {
        line := scanner.Text()
        domain, availability := parseOutput(line)
        
        switch domain {
        case "127.0.0.1":
            if availability != "67%" {
                t.Errorf("got availability of %s for 127.0.0.1, expected 67%%", availability)
            }
        case "127.0.0.2":
            if availability != "75%" {
                t.Errorf("got availability of %s for 127.0.0.2, expected 75%%", availability)
            }
        default:
            t.Errorf("got domain %s, expected either 127.0.0.1 or 127.0.0.2", domain)
        }

        i++
    }

    if i < 2 {
        t.Errorf("got %d lines of output, expected 2", i)
    }
}

func TestChangingAvailability(t *testing.T) {
    n_requests := 0

    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if n_requests == 1 || n_requests == 2 {
            w.WriteHeader(http.StatusInternalServerError)
        } else {
            w.WriteHeader(http.StatusOK)
        }

        n_requests++
    })

    url := "127.0.0.1:35580"
    l, err := net.Listen("tcp", url)
    if err != nil {
        t.Fatalf("failed to listen on %s", url)
    }

    server := httptest.NewUnstartedServer(handler)
    server.Listener.Close()
    server.Listener = l

    server.Start()
    defer server.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 70 * time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "go", "run", "main.go", "testdata/get.yaml")
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        t.Fatalf("failed to create stdout pipe")
    }

    go cmd.Run()

    percentages := [5]string{"100%", "50%", "33%", "50%", "60%"}

    i := 0
    scanner := bufio.NewScanner(stdout)
    for i < 5 && scanner.Scan() {
        line := scanner.Text()
        domain, availability := parseOutput(line)
        if domain != "127.0.0.1" || availability != percentages[i] {
            t.Errorf("got %s availability for %s, expected %s availability for 127.0.0.1", availability, domain, percentages[i])
        }

        i++
    }

    if i < 5 {
        t.Errorf("got only %d lines of availability info, expected 5", i)
    }
}

func TestComprehensive(t *testing.T) {
    validateLines := func(lines []string) {
        // check that each domain exists in exactly one line
        counts := [3]int{0, 0, 0}

        for _, line := range lines {
            domain, availability := parseOutput(line)

            switch domain {
            case "127.0.0.1":
                counts[0] += 1
                if availability != "85%" {
                    t.Errorf("got availability of %s for 127.0.0.1, expected 85%%", availability)
                }
            case "127.0.0.2":
                counts[1] += 1
                if availability != "75%" {
                    t.Errorf("got availability of %s for 127.0.0.2, expected 75%%", availability)
                }
            case "127.0.0.3":
                counts[2] += 1
                if availability != "90%" {
                    t.Errorf("got availability of %s for 127.0.0.3, expected 90%%", availability)
                }
            default:
                t.Errorf("got domain %s, expected either 127.0.0.1, 127.0.0.2, or 127.0.0.3", domain)
            }
        }

        for i := range 3 {
            if counts[i] != 1 {
                t.Errorf("one iteration of logging produce %d outputs for domain %d, expected 1", counts[i], i)
            }
        }
    }

    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(160 * time.Millisecond)

        if r.URL.Path == "/bad" {
            w.WriteHeader(http.StatusInternalServerError)
        } else {
            w.WriteHeader(http.StatusOK)
        }
    })

    for _, url := range [3]string{"127.0.0.1:35580", "127.0.0.2:35581", "127.0.0.3:35582"} {
        l, err := net.Listen("tcp", url)
        if err != nil {
            t.Fatalf("failed to listen on %s", url)
        }

        server := httptest.NewUnstartedServer(handler)
        server.Listener.Close()
        server.Listener = l

        server.Start()
        defer server.Close()
    }

    ctx, cancel := context.WithTimeout(context.Background(), 40 * time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "go", "run", "main.go", "testdata/comprehensive.yaml")
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        t.Fatalf("failed to create stdout pipe")
    }

    go cmd.Run()

    i := 0
    scanner := bufio.NewScanner(stdout)
    if scanner.Scan() {
        current_time := time.Now()

        lines := make([]string, 0)
        lines = append(lines, scanner.Text())
        j := 0
        for j < 2 && scanner.Scan() {
            lines = append(lines, scanner.Text())
            j++
        }

        validateLines(lines)
        i++

        for i < 3 && scanner.Scan() {
            prev_time := current_time
            current_time = time.Now()
            time_diff := current_time.Sub(prev_time).Milliseconds()
            if time_diff > 15400 || time_diff < 14600 {
                t.Errorf("time difference between intervals was %d ms, expected time to be within 14600ms - 15400ms", time_diff)
            }

            lines := make([]string, 0)
            lines = append(lines, scanner.Text())
            j := 0
            for j < 2 && scanner.Scan() {
                lines = append(lines, scanner.Text())
                j++
            }

            validateLines(lines)
            i++
        }
    }

    if i < 3 {
        t.Errorf("got only %d iterations of availability info, expected 3", i)
    }
}
