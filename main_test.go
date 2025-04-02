package main
import "testing"

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
