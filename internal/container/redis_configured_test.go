package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedisConfigured_ClusterUsesClusterAddrs(t *testing.T) {
	t.Setenv("REDIS_MODE", "cluster")
	t.Setenv("REDIS_CLUSTER_ADDRS", "10.0.0.1:6379,10.0.0.2:6379")
	t.Setenv("REDIS_ADDR", "")
	assert.True(t, redisConfigured())
}

func TestRedisConfigured_ClusterEmptyAddrs(t *testing.T) {
	t.Setenv("REDIS_MODE", "cluster")
	t.Setenv("REDIS_CLUSTER_ADDRS", " , ")
	t.Setenv("REDIS_ADDR", "ignored-for-gate")
	assert.False(t, redisConfigured())
}

func TestRedisConfigured_SingleUsesAddr(t *testing.T) {
	t.Setenv("REDIS_MODE", "single")
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("REDIS_CLUSTER_ADDRS", "")
	assert.True(t, redisConfigured())

	t.Setenv("REDIS_ADDR", "")
	assert.False(t, redisConfigured())
}
