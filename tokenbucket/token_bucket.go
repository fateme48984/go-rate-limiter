package tokenbucket

import (
	"sync"
	"time"
)

type TokenBucket struct {
	mu         sync.Mutex
	capacity   int
	refillRate float64
	userStates map[int]tokenState
}

type tokenState struct {
	lastRefillAt time.Time
	tokens       float64
}

func NewTokenBucket(capacity int, refillRate float64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		refillRate: refillRate,
		userStates: make(map[int]tokenState),
	}
}

func (tb *TokenBucket) Allow(userID int, now time.Time) bool {
	if tb.capacity <= 0 {
		return false
	}
	if tb.refillRate <= 0 {
		return false
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	state, exists := tb.userStates[userID]
	if !exists {
		state = tokenState{
			lastRefillAt: now,
			tokens:       float64(tb.capacity),
		}
	} else {
		if now.Before(state.lastRefillAt) {
			return false
		}

		elapsedSeconds := now.Sub(state.lastRefillAt).Seconds()
		refilledTokens := elapsedSeconds * tb.refillRate
		state.tokens = min(float64(tb.capacity), state.tokens+refilledTokens)
		state.lastRefillAt = now
	}

	if state.tokens < 1 {
		tb.userStates[userID] = state
		return false
	}

	state.tokens--
	tb.userStates[userID] = state
	return true
}
