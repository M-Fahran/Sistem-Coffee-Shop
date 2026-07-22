package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"coffeeshop/internal/config"
	"coffeeshop/internal/support/rateLimitter"
	"coffeeshop/internal/support/response"
)

func RateLimit(rdb *config.RedisClient, operation string, limit rateLimitter.RateLimit) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity := identifierFor(c)
		key := fmt.Sprintf("rl:%s:%s", operation, identity)

		allowed, remaining, retryAfter, err := slidingWindowCheck(
			c.Request.Context(),
			rdb,
			key,
			limit.Requests,
			limit.Window,
		)
		if err != nil {
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit.Requests))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(limit.Window).Unix(), 10))

		if !allowed {
			c.Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))

			c.AbortWithStatusJSON(
				http.StatusTooManyRequests,
				response.Error(
					http.StatusTooManyRequests,
					"RATE_LIMIT_429",
					"Too Many Requests",
					gin.H{
						"retry_after_seconds": int(retryAfter.Seconds()),
					},
				),
			)
			return
		}

		c.Next()
	}
}

func slidingWindowCheck(
	ctx context.Context,
	rdb *config.RedisClient,
	key string,
	maxRequests int,
	window time.Duration,
) (allowed bool, remaining int, retryAfter time.Duration, err error) {
	now := time.Now()
	windowStart := now.Add(-window).UnixNano()
	nowNano := now.UnixNano()

	uniqueMember := fmt.Sprintf("%d-%s", nowNano, randomHex(8))
	pipe := rdb.Pipeline()

	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))
	countCmd := pipe.ZCard(ctx, key)

	pipe.ZAdd(ctx, key, config.RedisZ{
		Score:  float64(nowNano),
		Member: uniqueMember,
	})

	pipe.Expire(ctx, key, window)

	if _, err = pipe.Exec(ctx); err != nil {
		return false, 0, 0, fmt.Errorf("redis pipeline: %w", err)
	}

	currentCount := int(countCmd.Val())
	if currentCount >= maxRequests {
		_ = rdb.ZRem(ctx, key, uniqueMember).Err()
		retryAfter = computeRetryAfter(ctx, rdb, key, window, now)
		return false, 0, retryAfter, nil
	}

	remaining = maxRequests - currentCount - 1
	return true, remaining, 0, nil
}

func computeRetryAfter(
	ctx context.Context,
	rdb *config.RedisClient,
	key string,
	window time.Duration,
	now time.Time,
) time.Duration {
	oldest, err := rdb.ZRangeWithScores(ctx, key, 0, 0).Result()
	if err != nil || len(oldest) == 0 {
		return window
	}

	oldestTime := time.Unix(0, int64(oldest[0].Score))
	retryAt := oldestTime.Add(window)
	retry := time.Until(retryAt)

	if retry < time.Second {
		return time.Second
	}

	return retry
}

func identifierFor(c *gin.Context) string {
	if userID, exist := c.Get("user_id"); exist && userID != nil {
		return fmt.Sprintf("user:%v", userID)
	}
	return fmt.Sprintf("ip:%s", c.ClientIP())
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Helper functions to create specific
func RateLimitLogin(rdb *config.RedisClient) gin.HandlerFunc {
	return RateLimit(rdb, "login", rateLimitter.RateLimitLogin)
}

func RateLimitLogout(rdb *config.RedisClient) gin.HandlerFunc {
	return RateLimit(rdb, "logout", rateLimitter.RateLimitLogout)
}

func RateLimitGet(rdb *config.RedisClient) gin.HandlerFunc {
	return RateLimit(rdb, "get", rateLimitter.RateLimitGet)
}

func RateLimitCreate(rdb *config.RedisClient) gin.HandlerFunc {
	return RateLimit(rdb, "create", rateLimitter.RateLimitCreate)
}

func RateLimitUpdate(rdb *config.RedisClient) gin.HandlerFunc {
	return RateLimit(rdb, "update", rateLimitter.RateLimitUpdate)
}

func RateLimitDelete(rdb *config.RedisClient) gin.HandlerFunc {
	return RateLimit(rdb, "delete", rateLimitter.RateLimitDelete)
}

func RateLimitPublic(rdb *config.RedisClient) gin.HandlerFunc {
	return RateLimit(rdb, "public", rateLimitter.RateLimitPublic)
}