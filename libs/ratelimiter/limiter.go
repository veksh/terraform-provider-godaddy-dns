package ratelimiter

import "context"

type Limiter interface {
	Wait()
	WaitCtx(ctx context.Context) error
}
