package slidingwindowlog

import (
	"slices"
	"sync"
	"time"
)

type SlidingWindowLog struct {
	mu          sync.Mutex
	userStates  map[int][]time.Time
	maxRequests int
	windowSize  time.Duration
}

func NewSlidingWindowLog(maxRequests int, windowSize time.Duration) *SlidingWindowLog {
	return &SlidingWindowLog{
		userStates:  make(map[int][]time.Time),
		maxRequests: maxRequests,
		windowSize:  windowSize,
	}
}

func (sw *SlidingWindowLog) Allow(userID int, now time.Time) bool {
	if sw.maxRequests <= 0 {
		return false
	}
	if sw.windowSize <= 0 {
		return false
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	requests, exists := sw.userStates[userID]
	if exists {
		lastRequestTime := requests[len(requests)-1]
		if now.Before(lastRequestTime) {
			return false
		}
	}
	cutoff := now.Add(-sw.windowSize)
	expired := 0

	for expired < len(requests) && !requests[expired].After(cutoff) {
		expired++
	}

	requests = slices.Delete(requests, 0, expired)

	if len(requests) >= sw.maxRequests {
		return false
	}

	sw.userStates[userID] = append(requests, now)

	return true
}
