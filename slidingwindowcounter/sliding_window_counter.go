package slidingwindowcounter

import (
	"sync"
	"time"
)

type windowState struct {
	previousBucketCount int
	currentBucketCount  int
	currentBucketStart  time.Time
}

type SlidingWindowCounter struct {
	mu          sync.Mutex
	userStates  map[int]windowState
	maxRequests int
	windowSize  time.Duration
}

func NewSlidingWindowCounter(maxRequests int, windowSize time.Duration) *SlidingWindowCounter {
	return &SlidingWindowCounter{
		userStates:  make(map[int]windowState),
		maxRequests: maxRequests,
		windowSize:  windowSize,
	}
}

func (sw *SlidingWindowCounter) Allow(userID int, now time.Time) bool {
	if sw.maxRequests <= 0 {
		return false
	}
	if sw.windowSize <= 0 {
		return false
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	userState, exist := sw.userStates[userID]
	newCurrentBucketStart := now.UTC().Truncate(sw.windowSize)
	previousBucketCnt := 0
	currentBucketCnt := 0
	if !exist {
		sw.userStates[userID] = windowState{
			previousBucketCount: previousBucketCnt,
			currentBucketCount:  currentBucketCnt + 1,
			currentBucketStart:  newCurrentBucketStart,
		}

		return true
	}

	if newCurrentBucketStart.Before(userState.currentBucketStart) {
		return false
	}

	bucketsElapsed := int(newCurrentBucketStart.Sub(userState.currentBucketStart) / sw.windowSize)

	switch {
	case bucketsElapsed == 0:
		//belongs to the current bucket
	case bucketsElapsed == 1:
		userState.previousBucketCount = userState.currentBucketCount
		userState.currentBucketCount = 0
		userState.currentBucketStart = newCurrentBucketStart
	default:
		userState.previousBucketCount = 0
		userState.currentBucketCount = 0
		userState.currentBucketStart = newCurrentBucketStart
	}

	elapsedInCurrentBucket := now.Sub(userState.currentBucketStart)
	weight := float64(sw.windowSize-elapsedInCurrentBucket) / float64(sw.windowSize)
	estimatedCount := float64(userState.previousBucketCount)*weight + float64(userState.currentBucketCount)

	candidateCount := estimatedCount + 1

	if candidateCount > float64(sw.maxRequests) {
		// Keep a completed bucket rotation even when this request is rejected.
		sw.userStates[userID] = userState
		return false
	}

	userState.currentBucketCount++
	sw.userStates[userID] = userState

	return true
}
