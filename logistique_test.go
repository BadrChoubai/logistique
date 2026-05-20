package logistique_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BadrChoubai/logistique"
)

func TestGateway_ProxiesRequest(t *testing.T) {
	// Fake upstream service
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify forwarded path
		if r.URL.Path != "/journal/entries" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("proxied response"))
	}))
	defer upstream.Close()

	// Create gateway
	gw, err := logistique.New(
		logistique.Config{},
		logistique.ServiceConfig{
			Prefix:  "/api/journal/",
			Target:  upstream.URL,
			Rewrite: "/journal",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	// Gateway test server
	server := httptest.NewServer(gw.Handler())
	defer server.Close()

	t.Log(server.URL)
	// Send request through gateway
	resp, err := http.Get(server.URL + "/api/journal/entries")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Verify status
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(body) != "proxied response" {
		t.Fatalf("unexpected body: %s", body)
	}
}
