package testutil

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultTestRedisURL = "redis://localhost:6379/15"

var redisCounter uint64

// NewRedisClient creates a Redis client and an isolated key prefix for tests.
// NewRedisClient 为测试创建 Redis 客户端与隔离 key 前缀。
func NewRedisClient(t testing.TB) (*redis.Client, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	redisURL := strings.TrimSpace(os.Getenv("KNOWLEDGE_CORE_TEST_REDIS_URL"))
	if redisURL == "" {
		redisURL = defaultTestRedisURL
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("parse test redis url failed: %v", err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Fatalf("ping redis failed: %v", err)
	}
	prefix := fmt.Sprintf("knowledge-core-test:%d:%d:%d", os.Getpid(), time.Now().UnixNano(), atomic.AddUint64(&redisCounter, 1))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		deleteRedisPrefix(cleanupCtx, client, prefix)
		_ = client.Close()
	})
	return client, prefix
}

func deleteRedisPrefix(ctx context.Context, client *redis.Client, prefix string) {
	var cursor uint64
	pattern := prefix + ":*"
	for {
		keys, next, err := client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return
		}
		if len(keys) > 0 {
			_ = client.Del(ctx, keys...).Err()
		}
		if next == 0 {
			return
		}
		cursor = next
	}
}
