package client

// put integration in separate file with // +build integration
// and run go test -v -tags=integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
)

const HTTPReplySometingCN = `
[{
	"data": "something.other.com",
	"name": "cn",
	"ttl": 3600,
	"type": "CNAME"
}]`

func TestGetRecords_ReturnsOneCname(t *testing.T) {
	t.Parallel()
	// also: NewTLSServer: tls, use ts.Client() to query it
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// http.ServeFile(w, r, "testdata/onecn.json")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, HTTPReplySometingCN)
		}))
	defer ts.Close()

	want := []model.DNSRecord{{
		Name: "cn",
		Type: "CNAME",
		Data: "something.other.com",
		TTL:  3600,
	}}
	c, err := NewClient(ts.URL, "dummyAPIKey", "dummyAPISecret")
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.GetRecords(context.Background(), "test.com", "CNAME", "cn")
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Error(cmp.Diff(want, got))
	}
}

func TestGetRecords_FailsOnError(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			httpReply := `this reply is malformed`
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, httpReply)
		}))
	defer ts.Close()

	c, err := NewClient(ts.URL, "dummyAPIKey", "dummyAPISecret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetRecords(context.Background(), "test.com", "CNAME", "cn")
	if err == nil {
		t.Fatal("got no error for malformed reply")
	}
}

func TestGetRecords_SetsAuthHeaders(t *testing.T) {
	t.Parallel()
	var authHeader string
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			authHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, HTTPReplySometingCN)
		}))
	defer ts.Close()
	c, err := NewClient(ts.URL, "dummyAPIKey", "dummyAPISecret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetRecords(context.Background(), "test.com", "CNAME", "cn")
	if err != nil {
		t.Fatal(err)
	}
	authHeaderWant := "sso-key dummyAPIKey:dummyAPISecret"
	if authHeader != authHeaderWant {
		t.Error("auth header mismatch: want", authHeaderWant, "got", authHeader)
	}
}

func TestGetRecords_RateLimit(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, HTTPReplySometingCN)
		}))
	defer ts.Close()

	c, err := NewClient(ts.URL, "dummyAPIKey", "dummyAPISecret")
	if err != nil {
		t.Fatal(err)
	}
	// not good to rely on params, but cumbersome to make it configurable :)
	start := time.Now()
	for i := 0; i < 61; i++ {
		_, err = c.GetRecords(context.Background(), "test.com", "CNAME", "cn")
		if err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)
	if elapsed < time.Second {
		t.Error("too little time elapsed for 61 request")
	}
}
