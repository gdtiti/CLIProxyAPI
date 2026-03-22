package distributedsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	storepkg "github.com/router-for-me/CLIProxyAPI/v6/internal/store"
	log "github.com/sirupsen/logrus"
)

// VersionStore captures the PG-backed store methods used by the distributed sync manager.
type VersionStore interface {
	GetDistributedVersions(ctx context.Context) (map[string]int64, error)
	ReloadConfigFromStore(ctx context.Context) (*storepkg.ConfigReloadResult, error)
	ReloadAuthFilesFromStore(ctx context.Context) (*storepkg.AuthReloadResult, error)
}

// RuntimeReloader captures watcher callbacks that refresh in-memory runtime state from disk.
type RuntimeReloader interface {
	ReloadConfigFromDisk() error
	ReloadAuthFiles(forceAuthRefresh bool) error
}

// EventSubscription yields distributed sync events from an external bus.
type EventSubscription interface {
	Events() <-chan storepkg.DistributedChange
	Close() error
}

// EventBus creates event subscriptions and manages the underlying bus lifecycle.
type EventBus interface {
	Subscribe(ctx context.Context) (EventSubscription, error)
	Close() error
}

// Manager coordinates Redis-triggered and polling-triggered distributed reloads.
type Manager struct {
	store        VersionStore
	runtime      RuntimeReloader
	bus          EventBus
	nodeID       string
	pollInterval time.Duration

	cancel   context.CancelFunc
	wg       sync.WaitGroup
	stopOnce sync.Once

	versionsMu    sync.Mutex
	localVersions map[string]int64

	subscription EventSubscription
}

// NewManager builds a distributed sync manager using Redis Pub/Sub plus PG version polling.
func NewManager(cfg config.DistributedSyncConfig, fallbackNodeID string, store VersionStore, runtime RuntimeReloader) (*Manager, error) {
	if store == nil {
		return nil, fmt.Errorf("distributed sync: store is required")
	}
	if runtime == nil {
		return nil, fmt.Errorf("distributed sync: runtime reloader is required")
	}

	var (
		bus EventBus
		err error
	)
	if cfg.IsConfigured() {
		bus, err = NewRedisEventBus(cfg)
		if err != nil {
			return nil, err
		}
	}

	return &Manager{
		store:        store,
		runtime:      runtime,
		bus:          bus,
		nodeID:       cfg.EffectiveNodeID(strings.TrimSpace(fallbackNodeID)),
		pollInterval: cfg.EffectivePollInterval(),
		localVersions: map[string]int64{
			storepkg.DistributedResourceConfig: 0,
			storepkg.DistributedResourceAuth:   0,
		},
	}, nil
}

// Start launches the initial reconcile, Redis subscription and poll loop.
func (m *Manager) Start(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	if err := m.reconcile(runCtx); err != nil {
		log.WithError(err).Warn("distributed sync: initial reconcile failed")
	}

	if m.bus != nil {
		sub, err := m.bus.Subscribe(runCtx)
		if err != nil {
			log.WithError(err).Warn("distributed sync: subscribe failed, using polling fallback only")
		} else if sub != nil {
			m.subscription = sub
			m.wg.Add(1)
			go m.consumeEvents(runCtx, sub)
		}
	}

	if m.pollInterval > 0 {
		m.wg.Add(1)
		go m.runPollLoop(runCtx)
	}

	return nil
}

// Stop stops the manager and closes Redis resources.
func (m *Manager) Stop() error {
	if m == nil {
		return nil
	}

	var stopErr error
	m.stopOnce.Do(func() {
		if m.cancel != nil {
			m.cancel()
		}
		if m.subscription != nil {
			if err := m.subscription.Close(); err != nil && stopErr == nil {
				stopErr = err
			}
		}
		if m.bus != nil {
			if err := m.bus.Close(); err != nil && stopErr == nil {
				stopErr = err
			}
		}
		m.wg.Wait()
	})

	return stopErr
}

func (m *Manager) consumeEvents(ctx context.Context, sub EventSubscription) {
	defer m.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-sub.Events():
			if !ok {
				return
			}
			if err := m.applyEvent(ctx, event); err != nil {
				log.WithError(err).Warnf(
					"distributed sync: failed to apply event resource=%s version=%d",
					event.Resource,
					event.Version,
				)
			}
		}
	}
}

func (m *Manager) runPollLoop(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.reconcile(ctx); err != nil {
				log.WithError(err).Warn("distributed sync: reconcile poll failed")
			}
		}
	}
}

func (m *Manager) applyEvent(ctx context.Context, event storepkg.DistributedChange) error {
	resource := strings.TrimSpace(event.Resource)
	if resource == "" || event.Version <= 0 {
		return nil
	}

	current := m.localVersion(resource)
	if event.NodeID != "" && event.NodeID == m.nodeID {
		if event.Version > current {
			m.setLocalVersion(resource, event.Version)
		}
		return nil
	}
	if event.Version <= current {
		return nil
	}
	if current > 0 && event.Version > current+1 {
		log.Warnf(
			"distributed sync: version gap detected resource=%s local=%d incoming=%d",
			resource,
			current,
			event.Version,
		)
	}
	if err := m.reloadResource(ctx, resource); err != nil {
		return err
	}
	m.setLocalVersion(resource, event.Version)
	return nil
}

func (m *Manager) reconcile(ctx context.Context) error {
	versions, err := m.store.GetDistributedVersions(ctx)
	if err != nil {
		return err
	}
	for _, resource := range []string{
		storepkg.DistributedResourceConfig,
		storepkg.DistributedResourceAuth,
	} {
		target := versions[resource]
		if target <= m.localVersion(resource) {
			continue
		}
		if err := m.reloadResource(ctx, resource); err != nil {
			return err
		}
		m.setLocalVersion(resource, target)
	}
	return nil
}

func (m *Manager) reloadResource(ctx context.Context, resource string) error {
	switch resource {
	case storepkg.DistributedResourceConfig:
		if _, err := m.store.ReloadConfigFromStore(ctx); err != nil {
			return err
		}
		return m.runtime.ReloadConfigFromDisk()
	case storepkg.DistributedResourceAuth:
		if _, err := m.store.ReloadAuthFilesFromStore(ctx); err != nil {
			return err
		}
		return m.runtime.ReloadAuthFiles(true)
	default:
		return nil
	}
}

func (m *Manager) localVersion(resource string) int64 {
	m.versionsMu.Lock()
	defer m.versionsMu.Unlock()
	return m.localVersions[resource]
}

func (m *Manager) setLocalVersion(resource string, version int64) {
	m.versionsMu.Lock()
	m.localVersions[resource] = version
	m.versionsMu.Unlock()
}

// DefaultNodeID returns a stable fallback node identifier.
func DefaultNodeID(port int) string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "cliproxy"
	}
	if port > 0 {
		return fmt.Sprintf("%s-%d", hostname, port)
	}
	return hostname
}

type redisEventBus struct {
	client  *redis.Client
	channel string
}

// NewRedisEventBus builds a Redis-backed event bus from the distributed sync config.
func NewRedisEventBus(cfg config.DistributedSyncConfig) (EventBus, error) {
	if !cfg.IsConfigured() {
		return nil, nil
	}
	client, err := newRedisClient(cfg)
	if err != nil {
		return nil, err
	}
	return &redisEventBus{
		client:  client,
		channel: cfg.EffectiveChannel(),
	}, nil
}

func (b *redisEventBus) Subscribe(ctx context.Context) (EventSubscription, error) {
	if b == nil || b.client == nil {
		return nil, nil
	}
	pubsub := b.client.Subscribe(ctx, b.channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, fmt.Errorf("distributed sync: subscribe redis channel: %w", err)
	}
	sub := newRedisSubscription(pubsub)
	sub.start()
	return sub, nil
}

func (b *redisEventBus) Close() error {
	if b == nil || b.client == nil {
		return nil
	}
	return b.client.Close()
}

type redisSubscription struct {
	pubsub *redis.PubSub
	events chan storepkg.DistributedChange
	once   sync.Once
}

func newRedisSubscription(pubsub *redis.PubSub) *redisSubscription {
	return &redisSubscription{
		pubsub: pubsub,
		events: make(chan storepkg.DistributedChange, 128),
	}
}

func (s *redisSubscription) start() {
	if s == nil || s.pubsub == nil {
		return
	}
	msgCh := s.pubsub.Channel(redis.WithChannelSize(128))
	go func() {
		defer close(s.events)
		for msg := range msgCh {
			if msg == nil {
				continue
			}
			var event storepkg.DistributedChange
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.WithError(err).Warn("distributed sync: ignore invalid redis event payload")
				continue
			}
			s.events <- event
		}
	}()
}

func (s *redisSubscription) Events() <-chan storepkg.DistributedChange {
	if s == nil {
		return nil
	}
	return s.events
}

func (s *redisSubscription) Close() error {
	if s == nil || s.pubsub == nil {
		return nil
	}
	var err error
	s.once.Do(func() {
		err = s.pubsub.Close()
	})
	return err
}
