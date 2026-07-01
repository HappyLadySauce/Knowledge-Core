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

const defaultRedisURL = "redis://localhost:6379/0"

var redisCounter uint64

// NewCacheClient creates a cache client and an isolated key prefix.
// NewCacheClient 创建缓存客户端与隔离 key 前缀。
func NewCacheClient(t testing.TB) (*redis.Client, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	redisURL := strings.TrimSpace(os.Getenv("KNOWLEDGE_CORE_REDIS_URL"))
	if redisURL == "" {
		redisURL = defaultRedisURL
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("parse redis url failed: %v", err)
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
