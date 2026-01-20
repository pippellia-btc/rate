// The package rate provides a concurrent implementation of a Token Bucket rate [Limiter].
// Each entity possesses its own dedicated, stateful token bucket,
// that is refilled according to the logic specified by the [Refiller].
package rate

import (
	"errors"
	"sync"
	"time"
)

// Limiter is a generic per-entity implementation of the Token Bucket algorithm.
// Entities can be any comparable type (e.g. strings, integers, floats, etc.).
type Limiter[K comparable] struct {
	mu       sync.RWMutex
	buckets  map[K]*Bucket
	refiller Refiller[K]
}

type Bucket struct {
	mu         sync.Mutex
	Tokens     float64
	LastRefill time.Time
}

// Refiller encapsulates the behaviour of the refill policy of the limiter.
// Users of this package can use custom refill policies by implementing this interface.
//
// For an example, see the [FlatRefiller] type.
type Refiller[K comparable] interface {
	// NewBucket creates a fully initialized Bucket object for a new entity.
	NewBucket(entity K) *Bucket

	// Refill updates the entity's bucket.
	// Before calling Refill, the [Limiter] will have already acquired the lock on the bucket.
	Refill(entity K, bucket *Bucket) error
}

// NewLimiter creates a new limiter with the refill policy encoded in the [Refiller].
func NewLimiter[K comparable](r Refiller[K]) *Limiter[K] {
	return &Limiter[K]{
		buckets:  make(map[K]*Bucket, 1000),
		refiller: r,
	}
}

// Reject returns true if the entity cannot pay the cost, false if it can.
func (l *Limiter[K]) Reject(entity K, cost float64) (bool, error) {
	allow, err := l.Allow(entity, cost)
	return !allow, err
}

// Allow returns true if the entity can pay the cost, false if it cannot.
func (l *Limiter[K]) Allow(entity K, cost float64) (bool, error) {
	if cost < 0 {
		return false, errors.New("cost must be non-negative")
	}
	if cost == 0 {
		return true, nil
	}

	l.mu.RLock()
	bucket, exists := l.buckets[entity]
	l.mu.RUnlock()

	if !exists {
		l.mu.Lock()
		// re-check while holding the write lock to avoid race conditions
		// where the same entity is assigned a bucket multiple times
		bucket, exists = l.buckets[entity]
		if !exists {
			bucket = l.refiller.NewBucket(entity)
			l.buckets[entity] = bucket
		}
		l.mu.Unlock()
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	if err := l.refiller.Refill(entity, bucket); err != nil {
		return false, err
	}

	if bucket.Tokens < cost {
		return false, nil
	}
	bucket.Tokens -= cost
	return true, nil
}

// FlatRefiller applies the same refill policy to every bucket.
// Every `Interval`, it refills `TokensPerInterval` without exceeding the `MaxTokens`.
type FlatRefiller[K comparable] struct {
	InitialTokens     float64
	MaxTokens         float64
	TokensPerInterval float64
	Interval          time.Duration
}

func (r FlatRefiller[K]) NewBucket(_ K) *Bucket {
	return &Bucket{
		Tokens:     r.InitialTokens,
		LastRefill: time.Now(),
	}
}

func (r FlatRefiller[K]) Refill(_ K, b *Bucket) error {
	if r.Interval <= 0 {
		return nil
	}

	refills := time.Since(b.LastRefill) / r.Interval
	if refills == 0 {
		return nil
	}

	b.Tokens = min(r.MaxTokens, b.Tokens+float64(refills)*r.TokensPerInterval)
	b.LastRefill = b.LastRefill.Add(refills * r.Interval)
	return nil
}
