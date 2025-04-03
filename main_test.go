package main
import (
    "bufio"
    "context"
    "fmt"
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

        body, _ :=  io.ReadAll(r.Body)
        fmt.Printf("BODY: %s\n", string(body))

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
