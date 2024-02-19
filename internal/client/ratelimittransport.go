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
	if err := t.limiter.WaitCtx(req.Context()); err != nil {
		return nil, err
	}
	return t.next.RoundTrip(req)
}
