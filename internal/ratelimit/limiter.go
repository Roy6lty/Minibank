package ratelimit

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Policy struct {
	Name            string
	Capacity        int64
	RefillPerSecond float64
}

type LoginAttemptPolicy struct {
	MaxFailures     int64
	FailureWindow   time.Duration
	LockoutDuration time.Duration
}

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

type Limiter interface {
	Allow(context.Context, string, Policy) (Decision, error)
}

type RedisConfig struct {
	Addr           string
	Password       string
	DB             int
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

type RedisLimiter struct {
	client *redis.Client
}

const tokenBucketScript = `
local capacity = tonumber(ARGV[1])
local refill_per_ms = tonumber(ARGV[2])
local ttl_seconds = tonumber(ARGV[3])

local server_time = redis.call("TIME")
local now_ms = (tonumber(server_time[1]) * 1000) + math.floor(tonumber(server_time[2]) / 1000)

local state = redis.call("HMGET", KEYS[1], "tokens", "timestamp")
local tokens = tonumber(state[1])
local timestamp = tonumber(state[2])

if tokens == nil then
    tokens = capacity
    timestamp = now_ms
end

if now_ms > timestamp then
    tokens = math.min(capacity, tokens + ((now_ms - timestamp) * refill_per_ms))
end

local allowed = 0
local retry_after = 0
if tokens >= 1 then
    tokens = tokens - 1
    allowed = 1
else
    retry_after = math.ceil((1 - tokens) / refill_per_ms)
end

redis.call("HSET", KEYS[1], "tokens", tokens, "timestamp", now_ms)
redis.call("EXPIRE", KEYS[1], ttl_seconds)

return {allowed, retry_after}
`

const checkLoginAttemptScript = `
local server_time = redis.call("TIME")
local now_ms = (tonumber(server_time[1]) * 1000) + math.floor(tonumber(server_time[2]) / 1000)
local locked_until = tonumber(redis.call("HGET", KEYS[1], "locked_until"))

if locked_until == nil then
    return 0
end
if locked_until > now_ms then
    return locked_until - now_ms
end

redis.call("HDEL", KEYS[1], "locked_until", "failures")
return 0
`

const recordLoginFailureScript = `
local max_failures = tonumber(ARGV[1])
local failure_ttl_seconds = tonumber(ARGV[2])
local lockout_ms = tonumber(ARGV[3])
local lockout_ttl_seconds = tonumber(ARGV[4])

local server_time = redis.call("TIME")
local now_ms = (tonumber(server_time[1]) * 1000) + math.floor(tonumber(server_time[2]) / 1000)
local locked_until = tonumber(redis.call("HGET", KEYS[1], "locked_until"))

if locked_until ~= nil and locked_until > now_ms then
    return locked_until - now_ms
end
if locked_until ~= nil then
    redis.call("HDEL", KEYS[1], "locked_until", "failures")
end

local failures = tonumber(redis.call("HGET", KEYS[1], "failures")) or 0
failures = failures + 1
redis.call("HSET", KEYS[1], "failures", failures)

if failures >= max_failures then
    locked_until = now_ms + lockout_ms
    redis.call("HSET", KEYS[1], "locked_until", locked_until)
    redis.call("EXPIRE", KEYS[1], math.max(failure_ttl_seconds, lockout_ttl_seconds))
    return lockout_ms
end

redis.call("EXPIRE", KEYS[1], failure_ttl_seconds)
return 0
`

func NewRedisLimiter(ctx context.Context, config RedisConfig) (*RedisLimiter, error) {
	if config.Addr == "" {
		return nil, fmt.Errorf("Redis address is required")
	}
	if config.ConnectTimeout <= 0 || config.ReadTimeout <= 0 || config.WriteTimeout <= 0 {
		return nil, fmt.Errorf("Redis timeouts must be positive")
	}

	client := redis.NewClient(&redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		DialTimeout:  config.ConnectTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	})

	pingCtx, cancel := context.WithTimeout(ctx, config.ConnectTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect to Redis: %w", err)
	}

	return &RedisLimiter{client: client}, nil
}

type RedisLoginAttemptTracker struct {
	client *redis.Client
	policy LoginAttemptPolicy
}

func (l *RedisLimiter) NewLoginAttemptTracker(
	policy LoginAttemptPolicy,
) (*RedisLoginAttemptTracker, error) {
	if policy.MaxFailures <= 0 {
		return nil, fmt.Errorf("login attempt max failures must be positive")
	}
	if policy.FailureWindow < time.Millisecond {
		return nil, fmt.Errorf("login attempt failure window must be at least 1ms")
	}
	if policy.LockoutDuration < time.Millisecond {
		return nil, fmt.Errorf("login attempt lockout duration must be at least 1ms")
	}
	return &RedisLoginAttemptTracker{
		client: l.client,
		policy: policy,
	}, nil
}

func (l *RedisLoginAttemptTracker) Check(
	ctx context.Context,
	key string,
) (time.Duration, error) {
	if key == "" {
		return 0, fmt.Errorf("login attempt key is required")
	}
	result, err := l.client.Eval(
		ctx,
		checkLoginAttemptScript,
		[]string{"login-fail:" + key},
	).Result()
	if err != nil {
		return 0, fmt.Errorf("check login protection: %w", err)
	}
	retryAfter, err := redisInt64(result)
	if err != nil {
		return 0, err
	}
	return time.Duration(retryAfter) * time.Millisecond, nil
}

func (l *RedisLoginAttemptTracker) RecordFailure(
	ctx context.Context,
	key string,
) (time.Duration, error) {
	if key == "" {
		return 0, fmt.Errorf("login attempt key is required")
	}
	result, err := l.client.Eval(
		ctx,
		recordLoginFailureScript,
		[]string{"login-fail:" + key},
		l.policy.MaxFailures,
		secondsForTTL(l.policy.FailureWindow),
		l.policy.LockoutDuration.Milliseconds(),
		secondsForTTL(l.policy.LockoutDuration),
	).Result()
	if err != nil {
		return 0, fmt.Errorf("record login failure: %w", err)
	}
	retryAfter, err := redisInt64(result)
	if err != nil {
		return 0, err
	}
	return time.Duration(retryAfter) * time.Millisecond, nil
}

func (l *RedisLoginAttemptTracker) Reset(
	ctx context.Context,
	key string,
) error {
	if key == "" {
		return fmt.Errorf("login attempt key is required")
	}
	if err := l.client.Del(ctx, "login-fail:"+key).Err(); err != nil {
		return fmt.Errorf("reset login failures: %w", err)
	}
	return nil
}

func secondsForTTL(duration time.Duration) int64 {
	seconds := int64(math.Ceil(duration.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}

func (l *RedisLimiter) Allow(ctx context.Context, key string, policy Policy) (Decision, error) {
	if key == "" {
		return Decision{}, fmt.Errorf("rate-limit key is required")
	}
	if policy.Name == "" || policy.Capacity <= 0 || policy.RefillPerSecond <= 0 {
		return Decision{}, fmt.Errorf("invalid rate-limit policy")
	}

	ttlSeconds := int64(math.Ceil(float64(policy.Capacity) / policy.RefillPerSecond))
	if ttlSeconds < 1 {
		ttlSeconds = 1
	}
	result, err := l.client.Eval(
		ctx,
		tokenBucketScript,
		[]string{"rate:" + policy.Name + ":" + key},
		policy.Capacity,
		policy.RefillPerSecond/1000,
		ttlSeconds,
	).Result()
	if err != nil {
		return Decision{}, fmt.Errorf("check rate limit: %w", err)
	}

	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return Decision{}, fmt.Errorf("invalid rate-limit response")
	}
	allowed, err := redisInt64(values[0])
	if err != nil {
		return Decision{}, err
	}
	retryAfter, err := redisInt64(values[1])
	if err != nil {
		return Decision{}, err
	}

	return Decision{
		Allowed:    allowed == 1,
		RetryAfter: time.Duration(retryAfter) * time.Millisecond,
	}, nil
}

func (l *RedisLimiter) Close() error {
	return l.client.Close()
}

func redisInt64(value interface{}) (int64, error) {
	switch value := value.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case string:
		return strconv.ParseInt(value, 10, 64)
	case []byte:
		return strconv.ParseInt(string(value), 10, 64)
	default:
		return 0, fmt.Errorf("invalid rate-limit value %T", value)
	}
}
