package config

import "time"

// RateLimit defines a single rate limit policy.
type RateLimit struct {
	Requests int
	Window   time.Duration
}

// Pre-defined limits per operation type.
var (
	RateLimitLogin  = RateLimit{Requests: 5, Window: time.Minute}
	RateLimitLogout = RateLimit{Requests: 30, Window: time.Minute}

	RateLimitGet = RateLimit{Requests: 60, Window: time.Minute}

	RateLimitCreate = RateLimit{Requests: 30, Window: time.Minute}
	RateLimitUpdate = RateLimit{Requests: 30, Window: time.Minute}

	RateLimitDelete = RateLimit{Requests: 20, Window: time.Minute}

	RateLimitPublic = RateLimit{Requests: 60, Window: time.Minute}
)