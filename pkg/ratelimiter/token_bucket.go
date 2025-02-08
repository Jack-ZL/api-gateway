package ratelimiter

import (
	"time"
)

// TokenBucketLimiter 令牌桶限流器
type TokenBucketLimiter struct {
	capacity          int
	tokens            int
	refillRate        int
	refillInterval    time.Duration
	lastRefillTime    time.Time
	refillPerInterval int
}

// NewTokenBucketLimiter 创建令牌桶限流器
func NewTokenBucketLimiter(requests int, interval time.Duration) *TokenBucketLimiter {
	if requests <= 0 {
		requests = 100 // 默认值
	}
	if interval <= 0 {
		interval = time.Second // 默认值
	}

	refillPerInterval := requests
	refillRate := int(float64(requests) / interval.Seconds())
	if refillRate <= 0 {
		refillRate = 1
		refillPerInterval = refillRate
		interval = time.Second / time.Duration(requests)
	}

	return &TokenBucketLimiter{
		capacity:          requests,
		tokens:            requests,
		refillRate:        refillRate,
		refillInterval:    interval,
		lastRefillTime:    time.Now(),
		refillPerInterval: refillPerInterval,
	}
}

// Allow 尝试获取令牌，成功返回 true，否则返回 false
func (limiter *TokenBucketLimiter) Allow() bool {
	limiter.refill()

	if limiter.tokens > 0 {
		limiter.tokens--
		return true
	}
	return false
}

// refill 令牌补充
func (limiter *TokenBucketLimiter) refill() {
	now := time.Now()
	elapsedTime := now.Sub(limiter.lastRefillTime)
	if elapsedTime >= limiter.refillInterval {
		intervals := int(elapsedTime / limiter.refillInterval)
		tokensToAdd := intervals * limiter.refillPerInterval
		limiter.tokens += tokensToAdd
		if limiter.tokens > limiter.capacity {
			limiter.tokens = limiter.capacity
		}
		limiter.lastRefillTime = now
	}
}
