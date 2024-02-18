package ratelimiter

import (
	"context"
	"errors"
	"sync"
	"time"
)

// simple token bucket rate limiter
// - keeps track of available tokens and the last time a token was added
// - on acquire: if there are tokens available, spend one and return immediately
// - if there are none
//   - check the time since a token was added, mb add some more
//   - spend one if now it is available
//   - if none, wait until next one will be available (while holding mutex)
type BucketRateLimiter struct {
	mu sync.Mutex
	// interval between tokens, s
	period time.Duration
	// total bucket size; 1 means "no bursts"
	bucketSize int
	// current number of tokens in bucket
	numTokens int
	// the time last token was added
	lastTokenTime time.Time
}

// context to cancel, rate per second, burst (bucket) size
func NewBucketRL(rate, burst int) (Limiter, error) {
	if rate <= 0 || burst <= 0 {
		return nil, errors.New("limiter rate and burst must be positive")
	}
	return &BucketRateLimiter{
		period:        time.Second / time.Duration(rate),
		bucketSize:    burst,
		numTokens:     burst,
		lastTokenTime: time.Now(),
	}, nil
}

// block until token becomes available
func (s *BucketRateLimiter) Wait() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// if bucket is empty: add tokens that are due, update last refill time
	if s.numTokens <= 0 {
		elapsedTime := time.Since(s.lastTokenTime)
		tokensToAdd := int(elapsedTime.Nanoseconds() / s.period.Nanoseconds())
		s.numTokens = tokensToAdd
		if s.numTokens > s.bucketSize {
			s.numTokens = s.bucketSize
		}
		s.lastTokenTime = s.lastTokenTime.Add(time.Duration(tokensToAdd) * s.period)
	}
	// if token available: ok, take it and release caller from the wait
	if s.numTokens > 0 {
		s.numTokens--
		return
	}
	// if still no tokens: will have to wait until nextTokenTime or cancel
	nextTokenTime := s.lastTokenTime.Add(s.period)
	waitDuration := time.Until(nextTokenTime)
	time.Sleep(waitDuration)
	// upd last token time and release from the wait
	s.lastTokenTime = nextTokenTime
}

// block until token becomes available, with cancellable context
func (s *BucketRateLimiter) WaitCtx(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// if bucket is empty: add tokens that are due, update last refill time
	if s.numTokens <= 0 {
		elapsedTime := time.Since(s.lastTokenTime)
		tokensToAdd := int(elapsedTime.Nanoseconds() / s.period.Nanoseconds())
		s.numTokens = tokensToAdd
		if s.numTokens > s.bucketSize {
			s.numTokens = s.bucketSize
		}
		s.lastTokenTime = s.lastTokenTime.Add(time.Duration(tokensToAdd) * s.period)
	}
	// if token available: ok, take it and release caller from the wait
	if s.numTokens > 0 {
		s.numTokens--
		return nil
	}
	// if still no tokens: will have to wait until nextTokenTime or cancel
	nextTokenTime := s.lastTokenTime.Add(s.period)
	waitDuration := time.Until(nextTokenTime)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitDuration):
	}
	// if not cancelled: upd last token time and release from the wait
	s.lastTokenTime = nextTokenTime
	return nil
}
