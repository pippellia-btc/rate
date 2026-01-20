# rate

A highly-concurrent, in-memory, generic token bucket rate limiter for Go.

## Features

- **Per-entity rate limiting**: each entity (user, IP, API key, etc.) gets its own token bucket
- **Thread-safe**: designed for high-concurrency workloads
- **Generic**: works with any comparable key type
- **Pluggable refill policies**: use the built-in `FlatRefiller` or implement your own

## Installation

```bash
go get github.com/pippellia-btc/rate
```

## Usage

### Basic Example with FlatRefiller

```go
package main

import (
    "fmt"
    "time"

    "github.com/pippellia-btc/rate"
)

func main() {
    // Allow 100 requests per minute, bursting up to 200
    refiller := rate.FlatRefiller[string]{
        InitialTokens:     200,
        MaxTokens:         200,
        TokensPerInterval: 100,
        Interval:          time.Minute,
    }

    limiter := rate.NewLimiter(refiller)

    allowed, err := limiter.Allow("user-123", 1)
    if err != nil {
        panic(err)
    }

    if allowed {
        fmt.Println("Request allowed")
    } else {
        fmt.Println("Rate limited")
    }
}
```

### Custom Refiller

Implement the `Refiller` interface for custom rate limiting logic:

```go
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
```

As an example, here is a Refiller that allows higher rates to premium users.

```go
type TieredRefiller struct {
    PremiumUsers []string
}

func (r TieredRefiller) NewBucket(userID string) *rate.Bucket {
    tokens := 100.0
    if slices.Contains(r.PremiumUsers, userID) {
        tokens = 1000.0
    }
    return &rate.Bucket{
        Tokens:     tokens,
        LastRefill: time.Now(),
    }
}

func (r TieredRefiller) Refill(userID string, b *rate.Bucket) error {
    elapsed := time.Since(b.LastRefill)
    if elapsed < time.Second {
        return nil
    }

    refillRate := 10.0 // 10 tokens/sec
    if slices.Contains(r.PremiumUsers, userID) {
        refillRate = 100.0
    }

    b.Tokens = min(1000, b.Tokens + elapsed.Seconds()*refillRate)
    b.LastRefill = time.Now()
    return nil
}
```
