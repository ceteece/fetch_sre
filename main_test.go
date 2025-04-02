package main
import (
    "net/http"
    "net/http/httptest"
    "testing"
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
        //print("RECEIVED REQUEST")
        w.WriteHeader(http.StatusOK)
        //print("WROTE HEADER")
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
        //{"basic_", Endpoint{"test", server.URL, "GET", make(map[string]string), ""}, 1, 1},
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
