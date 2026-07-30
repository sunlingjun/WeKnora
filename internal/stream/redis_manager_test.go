package stream

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

func TestNewStreamManagerRedisRequiresClient(t *testing.T) {
	t.Setenv("STREAM_MANAGER_TYPE", TypeRedis)
	_, err := NewStreamManager(nil)
	if err == nil {
		t.Fatal("expected error when STREAM_MANAGER_TYPE=redis and client is nil")
	}
}

func TestNewStreamManagerMemoryIgnoresNilRedis(t *testing.T) {
	t.Setenv("STREAM_MANAGER_TYPE", TypeMemory)
	sm, err := NewStreamManager(nil)
	if err != nil {
		t.Fatalf("memory manager: %v", err)
	}
	if _, ok := sm.(*MemoryStreamManager); !ok {
		t.Fatalf("got %T, want *MemoryStreamManager", sm)
	}
}

func TestRedisStreamManagerAppendAndGet(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	sm, err := NewRedisStreamManager(client, "stream:test", time.Hour)
	if err != nil {
		t.Fatalf("NewRedisStreamManager: %v", err)
	}

	ctx := context.Background()
	ev := interfaces.StreamEvent{
		ID:      "e1",
		Type:    types.ResponseTypeAnswer,
		Content: "你好",
	}
	if err := sm.AppendEvent(ctx, "sess", "msg", ev); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	events, next, err := sm.GetEvents(ctx, "sess", "msg", 0)
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if next != 1 || len(events) != 1 || events[0].Content != "你好" {
		t.Fatalf("got events=%+v next=%d", events, next)
	}
}

func TestNewStreamManagerUsesInjectedRedis(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	t.Setenv("STREAM_MANAGER_TYPE", TypeRedis)
	t.Setenv("REDIS_PREFIX", "stream:factory")
	// Ensure factory no longer depends on REDIS_ADDR for the client itself.
	os.Unsetenv("REDIS_ADDR")

	sm, err := NewStreamManager(client)
	if err != nil {
		t.Fatalf("NewStreamManager: %v", err)
	}
	if _, ok := sm.(*RedisStreamManager); !ok {
		t.Fatalf("got %T, want *RedisStreamManager", sm)
	}
}
