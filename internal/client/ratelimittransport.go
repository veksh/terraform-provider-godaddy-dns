package client

import (
	"net/http"

	"github.com/veksh/terraform-provider-godaddy-dns/libs/ratelimiter"
)

type rateLimitedHTTPTransport struct {
	limiter ratelimiter.Limiter
	next    http.RoundTripper
}

func (t *rateLimitedHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.limiter.WaitCtx(req.Context())
	return t.next.RoundTrip(req)
}
