package client

import (
	"net/http"

	"github.com/veksh/terraform-provider-godaddy-dns/libs/ratelimiter"
)

type rateLimitedHTTPTransport struct {
	limiter *ratelimiter.RateLimiter
	next    http.RoundTripper
}

func (t *rateLimitedHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.limiter.Wait()
	return t.next.RoundTrip(req)
}
