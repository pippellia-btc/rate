# rate

A highly-concurrent, in-memory, generic token bucket rate limiter for Go.

[![Go Report Card](https://goreportcard.com/badge/github.com/pippellia-btc/rate)](https://goreportcard.com/report/github.com/pippellia-btc/rate)
[![Go Reference](https://pkg.go.dev/badge/github.com/pippellia-btc/rate.svg)](https://pkg.go.dev/github.com/pippellia-btc/rate)

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

    if limiter.Allow("user-123", 1) {
        fmt.Println("Request allowed")
    } else {
        fmt.Println("Rate limited")
    }
}
```

### Penalize and Reward

You can adjust an entity's token balance based on external signals, without going through the normal `Allow` flow:

```go
// Penalize deducts tokens unconditionally, even going negative.
// Useful when an external system detects abuse (e.g., fraud detection, CAPTCHA failure).
limiter.Penalize("user-123", 50)

// Reward adds tokens unconditionally.
// Useful for good behavior (e.g., completing a CAPTCHA, verified account).
limiter.Reward("user-123", 20)
```

Unlike `Allow`, these methods:
- Do not trigger a refill before modifying the balance
- Can push the balance negative (`Penalize`) or above the normal max (`Reward`)
- Work even if the entity doesn't have a bucket yet (one is created automatically)

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
	Refill(entity K, bucket *Bucket)
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

func (r TieredRefiller) Refill(userID string, b *rate.Bucket) {
    elapsed := time.Since(b.LastRefill)
    if elapsed < time.Second {
        return
    }

    refillRate := 10.0 // 10 tokens/sec
    if slices.Contains(r.PremiumUsers, userID) {
        refillRate = 100.0
    }

    b.Tokens = min(1000, b.Tokens + elapsed.Seconds()*refillRate)
    b.LastRefill = time.Now()
}
```
