package rate

import "time"

// Refiller encapsulates the behaviour of the refill policy of the limiter.
// Users of this package can use custom refill policies by implementing this interface.
//
// For an example, see the [FlatRefiller] type.
type Refiller[K comparable] interface {
	// NewBucket creates a fully initialized Bucket object for a new entity.
	NewBucket(entity K) *Bucket

	// Refill updates the entity's bucket.
	// Before calling Refill, the [Limiter] will have already acquired the lock on the bucket.
	Refill(entity K, bucket *Bucket)
}

// NoRefill is a Refiller that does not refill the bucket after being created.
// It is useful for cases where the bucket is not supposed to be refilled.
type NoRefill[K comparable] struct {
	InitialTokens float64
}

func (r NoRefill[K]) NewBucket(_ K) *Bucket {
	return &Bucket{
		Tokens:     r.InitialTokens,
		LastRefill: time.Now(),
	}
}

func (r NoRefill[K]) Refill(_ K, _ *Bucket) {}

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

func (r FlatRefiller[K]) Refill(_ K, b *Bucket) {
	if r.Interval <= 0 {
		return
	}
	refills := time.Since(b.LastRefill) / r.Interval
	if refills == 0 {
		return
	}

	b.Tokens = min(r.MaxTokens, b.Tokens+float64(refills)*r.TokensPerInterval)
	b.LastRefill = b.LastRefill.Add(refills * r.Interval)
}
