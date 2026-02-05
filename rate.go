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

// Bucket is a stateful token bucket for an entity.
type Bucket struct {
	mu         sync.Mutex
	Tokens     float64
	LastRefill time.Time
}

// NewLimiter creates a new limiter with the refill policy encoded in the [Refiller].
func NewLimiter[K comparable](r Refiller[K]) *Limiter[K] {
	return &Limiter[K]{
		buckets:  make(map[K]*Bucket, 1000),
		refiller: r,
	}
}

// Allow returns true if the entity can afford the cost, false otherwise.
// If the cost is affordable, it is deducted from the entity's bucket.
//
// If the entity does not have a bucket yet, one is created via [Refiller.NewBucket].
// Before checking affordability, [Refiller.Refill] is called to replenish tokens.
//
// Use Allow to decide whether to process a request. To punish an entity after
// detecting abuse, use [Limiter.Penalize] instead.
func (l *Limiter[K]) Allow(entity K, cost float64) (bool, error) {
	if cost < 0 {
		return false, errors.New("limiter.Allow: cost must be non-negative")
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

// Penalize unconditionally deducts a cost from the entity's bucket.
// Unlike [Limiter.Allow], no refill is applied and the deduction always occurs,
// even if the resulting token balance becomes negative.
//
// If the entity does not have a bucket yet, one is created via [Refiller.NewBucket],
// then the penalty is applied. This allows punishing entities detected through
// external systems that may not have interacted with this limiter before.
//
// Use Penalize to punish an entity after detecting abuse. To check whether a
// request should be allowed, use [Limiter.Allow] instead.
func (l *Limiter[K]) Penalize(entity K, cost float64) error {
	if cost < 0 {
		return errors.New("limiter.Penalize: cost must be non-negative")
	}
	if cost == 0 {
		return nil
	}

	l.mu.RLock()
	bucket, exists := l.buckets[entity]
	l.mu.RUnlock()

	if !exists {
		// We don't consider penalizing an unknown entity as an error, because the abuse could
		// have happened elsewhere, without the limiter ever seeing the entity before.
		// So we create a new bucket for the entity, and then we proceed to penalize.
		l.mu.Lock()
		bucket, exists = l.buckets[entity]
		if !exists {
			bucket = l.refiller.NewBucket(entity)
			l.buckets[entity] = bucket
		}
		l.mu.Unlock()
	}

	bucket.mu.Lock()
	bucket.Tokens -= cost
	bucket.mu.Unlock()
	return nil
}
