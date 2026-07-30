package stream

import (
	"fmt"
	"os"
	"time"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/redis/go-redis/v9"
)

// 流管理器类型
const (
	TypeMemory = "memory"
	TypeRedis  = "redis"
)

// NewStreamManager 创建流管理器。
// Redis 模式复用 DI 注入的 UniversalClient（initRedisClient 已按 REDIS_MODE
// 创建单机或 Cluster 客户端），避免另起 redis.NewClient 只连 REDIS_ADDR 导致
// 集群环境下出现 MOVED。
func NewStreamManager(rdb redis.UniversalClient) (interfaces.StreamManager, error) {
	switch os.Getenv("STREAM_MANAGER_TYPE") {
	case TypeRedis:
		if rdb == nil {
			return nil, fmt.Errorf("STREAM_MANAGER_TYPE=redis requires a Redis client (check REDIS_ADDR / REDIS_MODE=cluster + REDIS_CLUSTER_ADDRS)")
		}
		ttl := time.Hour // 默认1小时
		return NewRedisStreamManager(rdb, os.Getenv("REDIS_PREFIX"), ttl)
	default:
		return NewMemoryStreamManager(), nil
	}
}
