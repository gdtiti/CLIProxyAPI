package distributedsync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	storepkg "github.com/router-for-me/CLIProxyAPI/v6/internal/store"
)

type redisPublishResult interface {
	Err() error
}

type redisPublisher interface {
	Publish(ctx context.Context, channel string, message any) redisPublishResult
	Close() error
}

type redisClientAdapter struct {
	client *redis.Client
}

func (a *redisClientAdapter) Publish(ctx context.Context, channel string, message any) redisPublishResult {
	return a.client.Publish(ctx, channel, message)
}

func (a *redisClientAdapter) Close() error {
	if a == nil || a.client == nil {
		return nil
	}
	return a.client.Close()
}

// RedisNotifier publishes committed store changes to Redis Pub/Sub.
type RedisNotifier struct {
	client  redisPublisher
	channel string
	nodeID  string
}

// NewRedisNotifier builds a Redis-backed notifier from the distributed sync config.
func NewRedisNotifier(cfg config.DistributedSyncConfig, fallbackNodeID string) (*RedisNotifier, error) {
	if !cfg.IsConfigured() {
		return nil, nil
	}
	client, err := newRedisClient(cfg)
	if err != nil {
		return nil, err
	}
	return newRedisNotifierWithClient(&redisClientAdapter{client: client}, cfg, fallbackNodeID), nil
}

func newRedisNotifierWithClient(client redisPublisher, cfg config.DistributedSyncConfig, fallbackNodeID string) *RedisNotifier {
	if client == nil {
		return nil
	}
	return &RedisNotifier{
		client:  client,
		channel: cfg.EffectiveChannel(),
		nodeID:  cfg.EffectiveNodeID(fallbackNodeID),
	}
}

// PublishDistributedChange implements store.DistributedChangeNotifier.
func (n *RedisNotifier) PublishDistributedChange(ctx context.Context, change storepkg.DistributedChange) error {
	if n == nil || n.client == nil {
		return nil
	}
	if strings.TrimSpace(change.Resource) == "" || change.Version <= 0 {
		return nil
	}

	if change.CreatedAt.IsZero() {
		change.CreatedAt = time.Now().UTC()
	}
	if change.NodeID == "" {
		change.NodeID = n.nodeID
	}

	data, err := json.Marshal(change)
	if err != nil {
		return fmt.Errorf("distributed sync: marshal redis event: %w", err)
	}
	if err = n.client.Publish(ctx, n.channel, data).Err(); err != nil {
		return fmt.Errorf("distributed sync: publish redis event: %w", err)
	}
	return nil
}

// Close closes the underlying Redis client.
func (n *RedisNotifier) Close() error {
	if n == nil || n.client == nil {
		return nil
	}
	return n.client.Close()
}

func newRedisClient(cfg config.DistributedSyncConfig) (*redis.Client, error) {
	if strings.TrimSpace(cfg.Redis.Addr) == "" {
		return nil, fmt.Errorf("distributed sync: redis addr is required")
	}
	return redis.NewClient(&redis.Options{
		Addr:     strings.TrimSpace(cfg.Redis.Addr),
		Username: strings.TrimSpace(cfg.Redis.Username),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}), nil
}
