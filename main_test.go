package main
import (
    "net/http"
    "net/http/httptest"
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

    // initialize stats
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
}
