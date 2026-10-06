package tokenbucket

import (
	"sync"
	"testing"
	"time"
)

func TestAllow_FirstRequestIsAllowed(t *testing.T) {
	limiter := NewTokenBucket(10, 0.2)
	allowed := limiter.Allow(1, testTime(12, 10, 10))

	if !allowed {
		t.Fatal("expected first request to be allowed")
	}
}

func TestAllow_RequestBeyondCapacityIsRejected(t *testing.T) {
	limiter := NewTokenBucket(2, 0.2)
	now := testTime(12, 10, 10)
	firstAllowed := limiter.Allow(1, now)
	secondAllowed := limiter.Allow(1, now)
	thirdAllowed := limiter.Allow(1, now)

	if !firstAllowed || !secondAllowed {
		t.Fatal("expected requests up to capacity to be allowed")
	}

	if thirdAllowed {
		t.Fatal("expected request beyond capacity to be rejected")
	}
}

func TestAllow_ZeroCapacityRejectsRequest(t *testing.T) {
	limiter := NewTokenBucket(0, 0.2)
	allowed := limiter.Allow(1, testTime(14, 23, 47))

	if allowed {
		t.Fatal("expected request to be rejected when capacity is zero")
	}
}

func TestAllow_NegativeCapacityRejectsRequest(t *testing.T) {
	limiter := NewTokenBucket(-1, 0.2)
	allowed := limiter.Allow(1, testTime(14, 23, 47))

	if allowed {
		t.Fatal("expected request to be rejected when capacity is negative")
	}
}

func TestAllow_ZeroRefillRateRejectsRequest(t *testing.T) {
	limiter := NewTokenBucket(10, 0.0)
	allowed := limiter.Allow(1, testTime(14, 23, 47))

	if allowed {
		t.Fatal("expected request to be rejected when refill rate is zero")
	}
}

func TestAllow_NegativeRefillRateRejectsRequest(t *testing.T) {
	limiter := NewTokenBucket(10, -0.1)
	allowed := limiter.Allow(1, testTime(14, 23, 47))

	if allowed {
		t.Fatal("expected request to be rejected when refill rate is negative")
	}
}

func TestAllow_RefillsTokensBasedOnElapsedTime(t *testing.T) {
	limiter := NewTokenBucket(1, 1)
	userID := 1
	bucketEmptiedAt := testTime(12, 10, 10)

	firstAllowed := limiter.Allow(userID, bucketEmptiedAt)
	requestWhileEmptyAllowed := limiter.Allow(userID, bucketEmptiedAt)
	requestAfterOneSecondAllowed := limiter.Allow(userID, bucketEmptiedAt.Add(time.Second))

	if !firstAllowed {
		t.Fatal("expected first request to be allowed")
	}
	if requestWhileEmptyAllowed {
		t.Fatal("expected request to be rejected while bucket is empty")
	}
	if !requestAfterOneSecondAllowed {
		t.Fatal("expected request to be allowed after one second refills one token")
	}
}

func TestAllow_PreservesFractionalTokensBetweenRequests(t *testing.T) {
	limiter := NewTokenBucket(1, 0.5)
	userID := 1
	bucketEmptiedAt := testTime(12, 10, 10)

	firstAllowed := limiter.Allow(userID, bucketEmptiedAt)

	if !firstAllowed {
		t.Fatal("expected first request to be allowed")
	}
	if limiter.userStates[userID].tokens != 0 {
		t.Fatal("expected tokens to be zero after first request")
	}

	requestAfterOneSecondAllowed := limiter.Allow(userID, bucketEmptiedAt.Add(time.Second))
	if requestAfterOneSecondAllowed {
		t.Fatal("expected request to be rejected when only half a token is available")
	}
	if limiter.userStates[userID].tokens != 0.5 {
		t.Fatalf("expected 0.5 token to be preserved, got %v", limiter.userStates[userID].tokens)
	}

	requestAfterTwoSecondsAllowed := limiter.Allow(userID, bucketEmptiedAt.Add(2*time.Second))
	if !requestAfterTwoSecondsAllowed {
		t.Fatal("expected two half-token refills to combine and allow the request")
	}
	if limiter.userStates[userID].tokens != 0 {
		t.Fatalf("expected combined token to be consumed, got %v remaining tokens", limiter.userStates[userID].tokens)
	}
}

func TestAllow_DoesNotRefillBeyondCapacity(t *testing.T) {
	limiter := NewTokenBucket(2, 0.2)
	userID := 1
	now := testTime(12, 10, 10)
	firstAllowed := limiter.Allow(userID, now)
	secondAllowed := limiter.Allow(userID, now)
	thirdAllowed := limiter.Allow(userID, now)

	requestAfterLongIdleAllowed := limiter.Allow(userID, testTime(13, 10, 13))

	if !firstAllowed || !secondAllowed || thirdAllowed {
		t.Fatal("expected requests up to capacity to be allowed and the next request to be rejected")
	}
	if !requestAfterLongIdleAllowed {
		t.Fatal("expected request after a long idle period to be allowed")
	}

	wantRemainingTokens := float64(limiter.capacity - 1)
	gotRemainingTokens := limiter.userStates[userID].tokens
	if gotRemainingTokens != wantRemainingTokens {
		t.Fatalf("expected %v remaining token after refill and request, got %v", wantRemainingTokens, gotRemainingTokens)
	}
}

func TestAllow_UsersHaveIndependentBuckets(t *testing.T) {
	limiter := NewTokenBucket(2, 0.2)
	now := testTime(12, 10, 10)
	userOneFirstAllowed := limiter.Allow(1, now)
	userOneSecondAllowed := limiter.Allow(1, now)
	userOneThirdAllowed := limiter.Allow(1, now)

	userTwoFirstAllowed := limiter.Allow(2, now)

	if !userOneFirstAllowed || !userOneSecondAllowed || userOneThirdAllowed {
		t.Fatal("expected user one's requests up to capacity to be allowed and the next request to be rejected")
	}

	if !userTwoFirstAllowed {
		t.Fatal("expected user two's first request to be allowed independently")
	}
}

func TestAllow_OlderTimestampIsRejectedWithoutChangingState(t *testing.T) {
	limiter := NewTokenBucket(10, 0.2)
	userID := 1
	firstAllowed := limiter.Allow(userID, testTime(14, 23, 47))
	stateBeforeOldRequest := limiter.userStates[userID]

	oldRequestTime := stateBeforeOldRequest.lastRefillAt.Truncate(time.Hour)
	oldRequestAllowed := limiter.Allow(userID, oldRequestTime)

	if !firstAllowed {
		t.Fatal("expected the first request to be allowed")
	}
	if oldRequestAllowed {
		t.Fatal("expected request with an older timestamp to be rejected")
	}

	stateAfterOldRequest := limiter.userStates[userID]
	if stateAfterOldRequest.tokens != stateBeforeOldRequest.tokens ||
		!stateAfterOldRequest.lastRefillAt.Equal(stateBeforeOldRequest.lastRefillAt) {
		t.Fatalf("expected older request not to change state; before=%+v after=%+v", stateBeforeOldRequest, stateAfterOldRequest)
	}
}

func TestAllow_ConcurrentRequestsRespectCapacity(t *testing.T) {
	capacity := 10
	totalRequests := 100
	refillRate := 0.1
	requestTime := testTime(12, 10, 10)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan bool, totalRequests)

	limiter := NewTokenBucket(capacity, refillRate)

	for range totalRequests {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Every goroutine waits here so the requests begin together.
			<-start
			// A shared timestamp prevents refill from adding tokens during this test.
			allowed := limiter.Allow(1, requestTime)
			results <- allowed
		}()
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

	if allowedCount != capacity {
		t.Fatalf(
			"expected %d allowed requests, got %d",
			capacity,
			allowedCount,
		)
	}
	if limiter.userStates[1].tokens != 0 {
		t.Fatalf("expected all %d tokens to be consumed, got %v remaining tokens", capacity, limiter.userStates[1].tokens)
	}
}

func testTime(hour, minute, second int) time.Time {
	return time.Date(2026, time.September, 28, hour, minute, second, 0, time.UTC)
}
