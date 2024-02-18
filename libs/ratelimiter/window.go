package ratelimiter

import (
	"context"
	"errors"
	"sync"
	"time"
)

// fixed winow rate limiter: limit number of request allowed per time interval
// - keeps track of interval start and last call time
// - on aquire:
//   - if last call was > interval length ago, refill bucket
//   - if there are tokens available, spend one and return immediately
//   - if not, wait until the end of interval, refill and spend one
//   - somewhat unpleasant side effect will be the burst of `bucketSize` requests
//     followed by the large wait for next: alternative is 1 RPS which is way slower
//     for shorter batches
type WindowRateLimiter struct {
	mu sync.Mutex
	// interval between bucket refills
	period time.Duration
	// total bucket size: num of requests per period
	bucketSize int
	// number of requests left for current period
	numTokens int
	// the time last bucket was refilled
	lastRefillTime time.Time
}

// period length and num of requests per period
func NewWindowRL(period time.Duration, RPP int) (Limiter, error) {
	if period <= 0 || RPP <= 0 {
		return nil, errors.New("limiter period and num of requests must be positive")
	}
	return &WindowRateLimiter{
		period:         period,
		bucketSize:     RPP,
		numTokens:      RPP,
		lastRefillTime: time.Now(),
	}, nil
}

// block until token becomes available
func (s *WindowRateLimiter) Wait() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// if last refilled happened long ago: refill now
	if time.Since(s.lastRefillTime) > s.period {
		s.lastRefillTime = time.Now()
		s.numTokens = s.bucketSize
	}
	// if bucket is empty: wait for next refill
	if s.numTokens == 0 {
		nextTokenTime := s.lastRefillTime.Add(s.period)
		waitDuration := time.Until(nextTokenTime)
		time.Sleep(waitDuration)

		s.lastRefillTime = time.Now()
		s.numTokens = s.bucketSize
	}
	s.numTokens--
	return
}

// block until token becomes available, with cancellable context
func (s *WindowRateLimiter) WaitCtx(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// if last refilled happened long ago: refill now
	if time.Since(s.lastRefillTime) > s.period {
		s.lastRefillTime = time.Now()
		s.numTokens = s.bucketSize
	}
	// if bucket is empty: wait for next refill
	if s.numTokens == 0 {
		nextTokenTime := s.lastRefillTime.Add(s.period)
		waitDuration := time.Until(nextTokenTime)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
		}

		s.lastRefillTime = time.Now()
		s.numTokens = s.bucketSize
	}
	s.numTokens--
	return nil
}
