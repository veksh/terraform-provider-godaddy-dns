package ratelimiter

import "context"

// GoDaddy current rate limiter: 60 requests in 60 seconds window per API endpoint
// https://developer.godaddy.com/getstarted#terms
// mb have separate limits per endpoint (get/set/del): need to see a request
type Limiter interface {
	Wait()
	WaitCtx(ctx context.Context) error
}
