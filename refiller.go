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
	Refill(entity K, bucket *Bucket) error
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
