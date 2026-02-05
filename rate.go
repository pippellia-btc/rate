// The package rate provides a concurrent implementation of a Token Bucket rate [Limiter].
// Each entity possesses its own dedicated, stateful token bucket,
// that is refilled according to the logic specified by the [Refiller].
package rate

import (
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

// Len returns the number of entities tracked by the limiter.
func (l *Limiter[K]) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.buckets)
}

// Balance returns the number of tokens in the entity's bucket.
// If the entity does not have a bucket yet, it returns 0.
func (l *Limiter[K]) Balance(entity K) float64 {
	l.mu.RLock()
	bucket, exists := l.buckets[entity]
	l.mu.RUnlock()

	if !exists {
		return 0
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	return bucket.Tokens
}

// Allow returns true if the entity can afford the cost, false otherwise.
// If the cost is affordable, it is deducted from the entity's bucket.
// It panics if the cost is negative.
//
// If the entity does not have a bucket yet, one is created via [Refiller.NewBucket].
// Before checking affordability, [Refiller.Refill] is called to replenish tokens.
//
// Use Allow to decide whether to process a request. To punish an entity after
// detecting abuse, use [Limiter.Penalize] instead.
func (l *Limiter[K]) Allow(entity K, cost float64) bool {
	if cost < 0 {
		panic("limiter.Allow: cost must be non-negative")
	}
	if cost == 0 {
		return true
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

	l.refiller.Refill(entity, bucket)
	if bucket.Tokens < cost {
		return false
	}
	bucket.Tokens -= cost
	return true
}

// Penalize unconditionally deducts a cost from the entity's bucket.
// Unlike [Limiter.Allow], no refill is applied and the deduction always occurs,
// even if the resulting token balance becomes negative.
// It panics if the cost is negative.
//
// If the entity does not have a bucket yet, one is created via [Refiller.NewBucket],
// then the penalty is applied. This allows punishing entities detected through
// external systems that may not have interacted with this limiter before.
//
// Use Penalize to punish an entity after detecting abuse. To check whether a
// request should be allowed, use [Limiter.Allow] instead.
func (l *Limiter[K]) Penalize(entity K, cost float64) {
	if cost < 0 {
		panic("limiter.Penalize: cost must be non-negative")
	}
	l.add(entity, -cost)
}

// Reward unconditionally adds a number of tokens to the entity's bucket.
// It panics if the reward is negative.
//
// If the entity does not have a bucket yet, one is created via [Refiller.NewBucket],
// then the reward is applied. This allows rewarding entities detected through
// external systems that may not have interacted with this limiter before.
//
// Use Reward to reward an entity after detecting good behaviour. To check whether a
// request should be allowed, use [Limiter.Allow] instead.
func (l *Limiter[K]) Reward(entity K, reward float64) {
	if reward < 0 {
		panic("limiter.Reward: reward must be non-negative")
	}
	l.add(entity, reward)
}

// Add unconditionally adds or deducts a number of tokens to the entity's bucket.
func (l *Limiter[K]) add(entity K, tokens float64) {
	if tokens == 0 {
		return
	}

	l.mu.RLock()
	bucket, exists := l.buckets[entity]
	l.mu.RUnlock()

	if !exists {
		// We don't consider adding tokens to an unknown entity as an error,
		// because the decision could have been made elsewhere. E.g. an external system detecting abuse.
		l.mu.Lock()
		bucket, exists = l.buckets[entity]
		if !exists {
			bucket = l.refiller.NewBucket(entity)
			l.buckets[entity] = bucket
		}
		l.mu.Unlock()
	}

	bucket.mu.Lock()
	bucket.Tokens += tokens
	bucket.mu.Unlock()
}
