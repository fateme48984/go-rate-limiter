# Rate Limiter in Go

A small Go project for implementing and comparing common rate-limiting algorithms, with a focus on concurrency and HTTP middleware.

## Algorithms

### Fixed Window
+ simple and cheap
- can allow bursts around window boundaries

### Sliding Window Log
+ accurate rolling window
- memory grows with request volume

### Sliding Window Counter
+ bounded memory
+ cheaper than Sliding Window Log
- approximate

### Token Bucket
+ supports controlled bursts
+ constant state per key
- bursts can still reach downstream services

### Leaky Bucket — Planned
+ smooths the output rate
+ protects downstream services from bursts
- adds queueing latency

## HTTP Integration

The rate limiters can be used as HTTP middleware and applied globally or to specific routes.

The current example uses a fixed user ID because authentication is outside the scope of this project.

## Concurrency & Tests

The implementations are designed to be safe under concurrent requests.

Tests use multiple goroutines to verify that configured limits are still respected under concurrent access.

## Run

```bash
go run .
```

Run tests:

```bash
go test ./...
```

Run with the race detector:

```bash
go test -race ./...
```