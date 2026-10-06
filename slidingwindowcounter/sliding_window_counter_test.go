package slidingwindowcounter

import (
	"sync"
	"testing"
	"time"
)

func TestAllow_ZeroMaxRequestsRejectsRequest(t *testing.T) {
	limiter := NewSlidingWindowCounter(0, time.Minute)
	allowed := limiter.Allow(1, testTime(14, 23, 47))

	if allowed {
		t.Fatal("expected request to be rejected when max requests is zero")
	}
}

func TestAllow_NegativeMaxRequestsRejectsRequest(t *testing.T) {
	limiter := NewSlidingWindowCounter(-1, time.Minute)
	allowed := limiter.Allow(1, testTime(14, 23, 47))

	if allowed {
		t.Fatal("expected request to be rejected when max requests is negative")
	}
}

func TestAllow_ZeroWindowSizeRejectsRequest(t *testing.T) {
	limiter := NewSlidingWindowCounter(10, 0)
	allowed := limiter.Allow(1, testTime(14, 23, 47))

	if allowed {
		t.Fatal("expected request to be rejected when window size is zero")
	}
}

func TestAllow_NegativeWindowSizeRejectsRequest(t *testing.T) {
	limiter := NewSlidingWindowCounter(10, -time.Minute)
	allowed := limiter.Allow(1, testTime(14, 23, 47))

	if allowed {
		t.Fatal("expected request to be rejected when window size is negative")
	}
}

func TestAllow_FirstRequestCreatesUserState(t *testing.T) {
	limiter := NewSlidingWindowCounter(10, time.Minute)
	userID := 1
	allowed := limiter.Allow(userID, testTime(14, 23, 47))

	if !allowed {
		t.Fatal("expected first request to be allowed")
	}
	assertUserState(t, limiter, userID, windowState{
		previousBucketCount: 0,
		currentBucketCount:  1,
		currentBucketStart:  testTime(14, 23, 0),
	})
}

func TestAllow_RequestsWithinSameBucketRespectLimit(t *testing.T) {
	limiter := NewSlidingWindowCounter(2, time.Minute)
	userID := 1

	firstAllowed := limiter.Allow(userID, testTime(14, 23, 10))
	secondAllowed := limiter.Allow(userID, testTime(14, 23, 20))
	thirdAllowed := limiter.Allow(userID, testTime(14, 23, 30))

	if !firstAllowed {
		t.Fatal("expected first request to be allowed")
	}
	if !secondAllowed {
		t.Fatal("expected second request at the limit to be allowed")
	}
	if thirdAllowed {
		t.Fatal("expected request beyond the limit to be rejected")
	}
}

func TestAllow_PreviousBucketWeightAllowsRequestLaterInNextBucket(t *testing.T) {
	limiter := NewSlidingWindowCounter(2, time.Minute)
	userID := 1

	firstAllowed := limiter.Allow(userID, testTime(14, 23, 47))
	secondAllowed := limiter.Allow(userID, testTime(14, 23, 47))
	nextBucketRequestAllowed := limiter.Allow(userID, testTime(14, 24, 30))

	if !firstAllowed || !secondAllowed {
		t.Fatal("expected both requests in previous bucket to be allowed")
	}
	if !nextBucketRequestAllowed {
		t.Fatal("expected request to be allowed after previous bucket weight decays")
	}
	assertUserState(t, limiter, userID, windowState{
		previousBucketCount: 2,
		currentBucketCount:  1,
		currentBucketStart:  testTime(14, 24, 0),
	})
}

func TestAllow_OneBucketElapsedRotatesCurrentBucketToPrevious(t *testing.T) {
	limiter := NewSlidingWindowCounter(4, time.Minute)
	userID := 1

	limiter.Allow(userID, testTime(14, 23, 10))
	limiter.Allow(userID, testTime(14, 23, 20))
	allowed := limiter.Allow(userID, testTime(14, 24, 30))

	if !allowed {
		t.Fatal("expected request in next bucket to be allowed")
	}
	assertUserState(t, limiter, userID, windowState{
		previousBucketCount: 2,
		currentBucketCount:  1,
		currentBucketStart:  testTime(14, 24, 0),
	})
}

func TestAllow_FullPreviousBucketRejectsRequestAtNextBucketStart(t *testing.T) {
	limiter := NewSlidingWindowCounter(2, time.Minute)
	userID := 1

	limiter.Allow(userID, testTime(14, 23, 10))
	limiter.Allow(userID, testTime(14, 23, 20))
	allowed := limiter.Allow(userID, testTime(14, 24, 0))

	if allowed {
		t.Fatal("expected request at next bucket start to be rejected while previous bucket is full")
	}
	assertUserState(t, limiter, userID, windowState{
		previousBucketCount: 2,
		currentBucketCount:  0,
		currentBucketStart:  testTime(14, 24, 0),
	})
}

func TestAllow_RequestAfterMultipleEmptyBucketsResetsStateAndIsAllowed(t *testing.T) {
	limiter := NewSlidingWindowCounter(3, time.Minute)
	userID := 1

	limiter.Allow(userID, testTime(14, 23, 10))
	limiter.Allow(userID, testTime(14, 23, 20))
	allowed := limiter.Allow(userID, testTime(14, 26, 12))

	if !allowed {
		t.Fatal("expected request after multiple empty buckets to be allowed")
	}
	assertUserState(t, limiter, userID, windowState{
		previousBucketCount: 0,
		currentBucketCount:  1,
		currentBucketStart:  testTime(14, 26, 0),
	})
}

func TestAllow_UsersHaveIndependentLimits(t *testing.T) {
	limiter := NewSlidingWindowCounter(1, time.Minute)
	now := testTime(14, 23, 10)

	userOneFirstAllowed := limiter.Allow(1, now)
	userOneSecondAllowed := limiter.Allow(1, now)
	userTwoFirstAllowed := limiter.Allow(2, now)

	if !userOneFirstAllowed {
		t.Fatal("expected user one's first request to be allowed")
	}
	if userOneSecondAllowed {
		t.Fatal("expected user one's second request to be rejected")
	}
	if !userTwoFirstAllowed {
		t.Fatal("expected user two's first request to be allowed independently")
	}
}

func TestAllow_OlderBucketTimestampIsRejectedWithoutChangingState(t *testing.T) {
	limiter := NewSlidingWindowCounter(2, time.Minute)
	userID := 1

	limiter.Allow(userID, testTime(14, 23, 10))
	allowed := limiter.Allow(userID, testTime(14, 22, 30))

	if allowed {
		t.Fatal("expected request from an older bucket to be rejected")
	}
	assertUserState(t, limiter, userID, windowState{
		previousBucketCount: 0,
		currentBucketCount:  1,
		currentBucketStart:  testTime(14, 23, 0),
	})
}

func TestAllow_NinetySecondWindowUsesConfiguredBucketBoundary(t *testing.T) {
	limiter := NewSlidingWindowCounter(2, 90*time.Second)
	userID := 1

	firstAllowed := limiter.Allow(userID, testTime(14, 23, 47))
	secondAllowed := limiter.Allow(userID, testTime(14, 23, 50))
	thirdAllowed := limiter.Allow(userID, testTime(14, 23, 55))

	if !firstAllowed || !secondAllowed {
		t.Fatal("expected first two requests in 90-second bucket to be allowed")
	}
	if thirdAllowed {
		t.Fatal("expected third request in 90-second bucket to be rejected")
	}
	assertUserState(t, limiter, userID, windowState{
		previousBucketCount: 0,
		currentBucketCount:  2,
		currentBucketStart:  testTime(14, 22, 30),
	})
}

func TestAllow_ConcurrentRequestsRespectLimit(t *testing.T) {
	maxRequests := 5
	windowSize := time.Minute
	totalRequests := 100
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan bool, totalRequests)

	limiter := NewSlidingWindowCounter(maxRequests, windowSize)

	minuteStart := testTime(14, 23, 0)
	requestTimes := make([]time.Time, totalRequests)

	for i := range totalRequests {
		second := i % 60
		requestTimes[i] = minuteStart.Add(
			time.Duration(second) * time.Second,
		)
	}

	for _, requestTime := range requestTimes {
		wg.Add(1)
		go func(requestTime time.Time) {
			defer wg.Done()

			<-start
			allowed := limiter.Allow(1, requestTime)
			results <- allowed
		}(requestTime)
	}

	close(start)
	wg.Wait()
	close(results)

	allowedCount := 0
	for value := range results {
		if value {
			allowedCount++
		}
	}

	if allowedCount != maxRequests {
		t.Fatalf(
			"expected %d allowed requests, got %d",
			maxRequests,
			allowedCount,
		)
	}
}

func testTime(hour, minute, second int) time.Time {
	return time.Date(2026, time.September, 28, hour, minute, second, 0, time.UTC)
}

func assertUserState(t *testing.T, limiter *SlidingWindowCounter, userID int, want windowState) {
	t.Helper()

	got, exists := limiter.userStates[userID]
	if !exists {
		t.Fatalf("expected state for user %d to exist", userID)
	}
	if got.previousBucketCount != want.previousBucketCount {
		t.Fatalf("expected previous bucket count %d, got %d", want.previousBucketCount, got.previousBucketCount)
	}
	if got.currentBucketCount != want.currentBucketCount {
		t.Fatalf("expected current bucket count %d, got %d", want.currentBucketCount, got.currentBucketCount)
	}
	if !got.currentBucketStart.Equal(want.currentBucketStart) {
		t.Fatalf("expected current bucket start %s, got %s", want.currentBucketStart, got.currentBucketStart)
	}
}
