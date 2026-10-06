package fixedwindow

import (
	"sync"
	"time"
)

type windowState struct {
	windowStart time.Time
	count       int
}

type FixedWindow struct {
	mu          sync.Mutex // to protect userStates
	userStates  map[int]windowState
	maxRequests int
}

// userStates[123] = {
//		windowStart: 02:01:00
//      count:       3}

func NewFixedWindow(maxRequests int) *FixedWindow {
	return &FixedWindow{
		userStates:  make(map[int]windowState),
		maxRequests: maxRequests,
	}
}

func (fx *FixedWindow) Allow(userID int, now time.Time) bool {
	if fx.maxRequests <= 0 {
		return false
	}

	currentWindowStart := now.UTC().Truncate(time.Minute)
	fx.mu.Lock()
	defer fx.mu.Unlock()

	val, ok := fx.userStates[userID]
	count := 1
	if ok {
		if val.windowStart.Equal(currentWindowStart) {
			if val.count >= fx.maxRequests {
				return false
			}
			val.count++
			count = val.count
		}
	}

	fx.userStates[userID] = windowState{
		windowStart: currentWindowStart,
		count:       count,
	}

	return true
}
