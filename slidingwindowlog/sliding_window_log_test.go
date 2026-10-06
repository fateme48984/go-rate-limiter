package slidingwindowlog

import (
	"sync"
	"testing"
	"time"
)

func TestAllow_ZeroMaxRequestsRejectsRequest(t *testing.T) {
	limiter := NewSlidingWindowLog(0, time.Minute)
	allowed := limiter.Allow(1, logTestTime(14, 23, 47))

	if allowed {
		t.Fatal("expected request to be rejected when max requests is zero")
	}
}

func TestAllow_NegativeMaxRequestsRejectsRequest(t *testing.T) {
	limiter := NewSlidingWindowLog(-1, time.Minute)
	allowed := limiter.Allow(1, logTestTime(14, 23, 47))

	if allowed {
		t.Fatal("expected request to be rejected when max requests is negative")
	}
}

func TestAllow_ZeroWindowSizeRejectsRequest(t *testing.T) {
	limiter := NewSlidingWindowLog(10, 0)
	allowed := limiter.Allow(1, logTestTime(14, 23, 47))

	if allowed {
		t.Fatal("expected request to be rejected when window size is zero")
	}
}

func TestAllow_NegativeWindowSizeRejectsRequest(t *testing.T) {
	limiter := NewSlidingWindowLog(10, -time.Minute)
	allowed := limiter.Allow(1, logTestTime(14, 23, 47))

	if allowed {
		t.Fatal("expected request to be rejected when window size is negative")
	}
}

func TestAllow_FirstRequestIsAllowedAndStored(t *testing.T) {
	limiter := NewSlidingWindowLog(2, time.Minute)
	userID := 1
	now := logTestTime(14, 23, 47)

	allowed := limiter.Allow(userID, now)

	if !allowed {
		t.Fatal("expected first request to be allowed")
	}
	assertRequestTimes(t, limiter, userID, []time.Time{now})
}

func TestAllow_RequestsWithinWindowRespectLimit(t *testing.T) {
	limiter := NewSlidingWindowLog(2, time.Minute)
	userID := 1
	windowStart := logTestTime(14, 23, 0)

	firstAllowed := limiter.Allow(userID, windowStart.Add(10*time.Second))
	secondAllowed := limiter.Allow(userID, windowStart.Add(30*time.Second))
	thirdAllowed := limiter.Allow(userID, windowStart.Add(50*time.Second))

	if !firstAllowed || !secondAllowed {
		t.Fatal("expected requests up to the limit to be allowed")
	}
	if thirdAllowed {
		t.Fatal("expected request beyond the limit to be rejected")
	}
	assertRequestTimes(t, limiter, userID, []time.Time{
		windowStart.Add(10 * time.Second),
		windowStart.Add(30 * time.Second),
	})
}

func TestAllow_ExpiredRequestsAreRemovedAndCapacityIsRestored(t *testing.T) {
	limiter := NewSlidingWindowLog(2, time.Minute)
	userID := 1
	firstRequestTime := logTestTime(14, 23, 0)
	secondRequestTime := firstRequestTime.Add(30 * time.Second)
	requestAfterExpirationTime := firstRequestTime.Add(61 * time.Second)

	firstAllowed := limiter.Allow(userID, firstRequestTime)
	secondAllowed := limiter.Allow(userID, secondRequestTime)
	requestAfterExpirationAllowed := limiter.Allow(userID, requestAfterExpirationTime)

	if !firstAllowed || !secondAllowed {
		t.Fatal("expected initial requests up to the limit to be allowed")
	}
	if !requestAfterExpirationAllowed {
		t.Fatal("expected request to be allowed after the oldest request expires")
	}
	assertRequestTimes(t, limiter, userID, []time.Time{
		secondRequestTime,
		requestAfterExpirationTime,
	})
}

func TestAllow_RequestExactlyAtWindowBoundaryIsAllowed(t *testing.T) {
	limiter := NewSlidingWindowLog(1, time.Minute)
	userID := 1
	firstRequestTime := logTestTime(14, 23, 0)
	boundaryTime := firstRequestTime.Add(time.Minute)

	firstAllowed := limiter.Allow(userID, firstRequestTime)
	boundaryRequestAllowed := limiter.Allow(userID, boundaryTime)

	if !firstAllowed {
		t.Fatal("expected first request to be allowed")
	}
	if !boundaryRequestAllowed {
		t.Fatal("expected request exactly one window later to be allowed")
	}
	assertRequestTimes(t, limiter, userID, []time.Time{boundaryTime})
}

func TestAllow_UsersHaveIndependentLimits(t *testing.T) {
	limiter := NewSlidingWindowLog(1, time.Minute)
	now := logTestTime(14, 23, 10)

	userOneFirstAllowed := limiter.Allow(1, now)
	userOneSecondAllowed := limiter.Allow(1, now)
	userTwoFirstAllowed := limiter.Allow(2, now)

	if !userOneFirstAllowed {
		t.Fatal("expected user one's first request to be allowed")
	}
	if userOneSecondAllowed {
		t.Fatal("expected user one's request beyond the limit to be rejected")
	}
	if !userTwoFirstAllowed {
		t.Fatal("expected user two's first request to be allowed independently")
	}
}

func TestAllow_OlderTimestampIsRejectedWithoutChangingState(t *testing.T) {
	limiter := NewSlidingWindowLog(2, time.Minute)
	userID := 1
	firstRequestTime := logTestTime(14, 23, 47)
	olderRequestTime := firstRequestTime.Add(-time.Second)

	firstAllowed := limiter.Allow(userID, firstRequestTime)
	olderRequestAllowed := limiter.Allow(userID, olderRequestTime)

	if !firstAllowed {
		t.Fatal("expected first request to be allowed")
	}
	if olderRequestAllowed {
		t.Fatal("expected request with an older timestamp to be rejected")
	}
	assertRequestTimes(t, limiter, userID, []time.Time{firstRequestTime})
}

func TestAllow_ConcurrentRequestsRespectLimit(t *testing.T) {
	maxRequests := 10
	totalRequests := 100
	requestTime := logTestTime(14, 23, 10)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan bool, totalRequests)

	limiter := NewSlidingWindowLog(maxRequests, time.Minute)

	for range totalRequests {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Closing start releases every goroutine at approximately the same time.
			<-start
			results <- limiter.Allow(1, requestTime)
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	allowedCount := 0
	for allowed := range results {
		if allowed {
			allowedCount++
		}
	}

	if allowedCount != maxRequests {
		t.Fatalf("expected %d allowed requests, got %d", maxRequests, allowedCount)
	}
	if got := len(limiter.userStates[1]); got != maxRequests {
		t.Fatalf("expected %d stored request timestamps, got %d", maxRequests, got)
	}
}

func logTestTime(hour, minute, second int) time.Time {
	return time.Date(2026, time.September, 28, hour, minute, second, 0, time.UTC)
}

func assertRequestTimes(
	t *testing.T,
	limiter *SlidingWindowLog,
	userID int,
	want []time.Time,
) {
	t.Helper()

	got, exists := limiter.userStates[userID]
	if !exists {
		t.Fatalf("expected request log for user %d to exist", userID)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d stored request timestamps, got %d: %v", len(want), len(got), got)
	}
	for index := range want {
		if !got[index].Equal(want[index]) {
			t.Fatalf("expected timestamp %d to be %s, got %s", index, want[index], got[index])
		}
	}
}
