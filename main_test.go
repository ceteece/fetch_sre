package main
import (
    "bufio"
    "context"
    "fmt"
    //"io"
    "net"
    "net/http"
    "net/http/httptest"
    "os/exec"
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

func TestIntegrationBasic(t *testing.T) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

    ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "go", "run", "main.go", "testdata/basic.yaml")
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        t.Fatalf("failed to create stdout pipe")
    }

    //stderr, err := cmd.StderrPipe()
    //if err != nil {
    //    t.Fatalf("failed to create stderr pipe")
    //}

    //err = cmd.Run()
    //if err != nil {
    //    t.Fatalf("failed to start command")
    //}

    cmd.Start()

    scanner := bufio.NewScanner(stdout)
    for scanner.Scan() {
        line := scanner.Text()
        fmt.Println(line)
    }

    //output, err := io.ReadAll(stdout)
    //if err != nil {
    //    t.Fatalf("failed to read stdout")
    //}

    //errors, err := io.ReadAll(stderr)
    //if err != nil {
    //    t.Fatalf("failed to read stderr")
    //}

    //fmt.Println("stdout: ", string(output))
    //fmt.Println("stderr: ", string(errors))
}
