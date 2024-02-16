package ratelimiter

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	// number of seconds required to refresh all tokens
	TOKEN_REFRESH_AS_SECONDS = time.Second * 60
)

// simple token bucket rate limiter
// - keeps track of available tokens and the last time a token was added
// - on acquire: if there are tokens available, spend one and return immediately
// - if there are none
//   - check the time since a token was added, mb add some more
//   - spend one if now it is available
//   - if none, wait until next one will be available (while holding mutex)
type RateLimiter struct {
	mu sync.Mutex
	// total bucket size; 1 means "no bursts"
	bucketSize int
	// current number of tokens in bucket
	numTokens int
	// the time the first token was used in this bucket
	firstTokenUsedTime time.Time
}

// context to cancel, rate per second, burst (bucket) size
func New(rate, burst int) (*RateLimiter, error) {
	if rate <= 0 || burst <= 0 {
		return nil, errors.New("limiter rate and burst must be positive")
	}
	return &RateLimiter{
		bucketSize:    burst,
		numTokens:     burst,
		firstTokenUsedTime: time.Now(),
	}, nil
}

// block until token becomes available
func (s *RateLimiter) Wait() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// if bucket is empty: wait for TOKEN_REFRESH_AS_SECONDS seconds since the first token was used, refill the bucket to (bucketSize),
	// and set the first token time to time.Now() to set the new start of the period.
	// this will help us comply with the GoDaddy APIs, which limit us to 60 requests every minute per endpoint.
	// See: https://developer.godaddy.com/getstarted#terms
	if s.numTokens <= 0 {

		// if fewer than TOKEN_REFRESH_AS_SECONDS seconds have passed since firstTokenUsedTime,
		// calculate firstTokenUsedTime + TOKEN_REFRESH_AS_SECONDS and sleep until that time
		if (time.Since(s.firstTokenUsedTime) <= TOKEN_REFRESH_AS_SECONDS) {
			minimumRefreshTime := s.firstTokenUsedTime.Add(TOKEN_REFRESH_AS_SECONDS)
			requiredTokenRefreshDuration := time.Since(minimumRefreshTime).Abs()

			if (requiredTokenRefreshDuration > TOKEN_REFRESH_AS_SECONDS) {
				// ensure we only sleep for TOKEN_REFRESH_AS_SECONDS seconds maximum
				requiredTokenRefreshDuration = TOKEN_REFRESH_AS_SECONDS
			}
			time.Sleep(requiredTokenRefreshDuration)
		}

		// now reset tokens and set first token use time
		s.numTokens = s.bucketSize
		s.firstTokenUsedTime = time.Now()
	} else {
		// if token available: ok, take it and release caller from the wait
		s.numTokens--
		return
	}
}

// block until token becomes available, with cancellable context
func (s *RateLimiter) WaitCtx(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// if bucket is empty: wait for TOKEN_REFRESH_AS_SECONDS seconds since the last token was used, refill the bucket to (bucketSize),
	// and set the last token time to time.Now() to set the new start of the period.
	// this is unique to the GoDaddy APIs, which limit us to 60 requests every minute per endpoint.
	// See: https://developer.godaddy.com/getstarted#terms
	if s.numTokens <= 0 {
		// if fewer than TOKEN_REFRESH_AS_SECONDS seconds have passed since firstTokenUsedTime,
		// calculate firstTokenUsedTime + TOKEN_REFRESH_AS_SECONDS and sleep until that time
		requiredTokenRefreshDuration := time.Second * 0
		if (time.Since(s.firstTokenUsedTime) <= TOKEN_REFRESH_AS_SECONDS) {
			minimumRefreshTime := s.firstTokenUsedTime.Add(TOKEN_REFRESH_AS_SECONDS)
			requiredTokenRefreshDuration = time.Since(minimumRefreshTime).Abs()

			if (requiredTokenRefreshDuration > TOKEN_REFRESH_AS_SECONDS) {
				// ensure we only sleep for TOKEN_REFRESH_AS_SECONDS seconds maximum
				requiredTokenRefreshDuration = TOKEN_REFRESH_AS_SECONDS
			}

			if (requiredTokenRefreshDuration < 0) {
				// don't wait for a negative amount of time. This is necessary since time.After() doesn't instantly
				// return on a negative duration.
				requiredTokenRefreshDuration = time.Second * 0
			}
		}

		s.numTokens = s.bucketSize

		select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(requiredTokenRefreshDuration):
		}
		// if not cancelled: upd last token time and release from the wait
		s.firstTokenUsedTime = time.Now()
		return nil
	}
	// if token available: ok, take it and release caller from the wait
	if s.numTokens > 0 {
		s.numTokens--
		return nil
	}
	return nil
}
