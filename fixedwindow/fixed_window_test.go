package fixedwindow

import (
	"sync"
	"testing"
	"time"
)

func TestAllow_FirstRequestIsAllowed(t *testing.T) {
	limiter := NewFixedWindow(10)

	now := time.Date(
		2026,
		time.September,
		28,
		14,
		23,
		47,
		0,
		time.UTC,
	)

	allowed := limiter.Allow(1, now)

	if !allowed {
		t.Fatal("expected first request to be allowed")
	}
}

func TestAllow_DifferentTimesWithinSameWindowShareLimit(t *testing.T) {
	limiter := NewFixedWindow(2)

	firstAllowed := limiter.Allow(1, time.Date(
		2026,
		time.September,
		28,
		14,
		23,
		12,
		0,
		time.UTC,
	))
	secondAllowed := limiter.Allow(1, time.Date(
		2026,
		time.September,
		28,
		14,
		23,
		47,
		0,
		time.UTC,
	))
	thirdAllowed := limiter.Allow(1, time.Date(
		2026,
		time.September,
		28,
		14,
		23,
		55,
		0,
		time.UTC,
	))

	if !firstAllowed {
		t.Fatal("expected first request to be allowed")
	}
	if !secondAllowed {
		t.Fatal("expected second request to be allowed")
	}

	if thirdAllowed {
		t.Fatal("expected third request to be rejected")
	}

}

func TestAllow_RequestBeyondLimitIsRejected(t *testing.T) {
	limiter := NewFixedWindow(2)

	now := time.Date(
		2026,
		time.September,
		28,
		14,
		23,
		47,
		0,
		time.UTC,
	)

	firstAllowed := limiter.Allow(1, now)
	secondAllowed := limiter.Allow(1, now)
	thirdAllowed := limiter.Allow(1, now)

	if !firstAllowed {
		t.Fatal("expected first request to be allowed")
	}

	if !secondAllowed {
		t.Fatal("expected second request to be allowed")
	}

	if thirdAllowed {
		t.Fatal("expected third request to be rejected")
	}
}

func TestAllow_ZeroLimitRejectsRequest(t *testing.T) {
	limiter := NewFixedWindow(0)

	now := time.Date(
		2026,
		time.September,
		28,
		14,
		23,
		47,
		0,
		time.UTC,
	)

	allowed := limiter.Allow(1, now)

	if allowed {
		t.Fatal("expected request to be rejected when max requests is zero")
	}
}
func TestAllow_NegativeLimitRejectsRequest(t *testing.T) {
	limiter := NewFixedWindow(-1)

	now := time.Date(
		2026,
		time.September,
		28,
		14,
		23,
		47,
		0,
		time.UTC,
	)

	allowed := limiter.Allow(1, now)

	if allowed {
		t.Fatal("expected request to be rejected when max requests is negative")
	}
}

func TestAllow_NewWindowResetsLimit(t *testing.T) {
	maxRequests := 2
	limiter := NewFixedWindow(maxRequests)

	previousWindowTime := time.Date(
		2026,
		time.September,
		28,
		14,
		23,
		47,
		0,
		time.UTC,
	)

	previousFirstAllowed := limiter.Allow(1, previousWindowTime)
	previousSecondAllowed := limiter.Allow(1, previousWindowTime)
	previousThirdAllowed := limiter.Allow(1, previousWindowTime)
	newWindowTime := time.Date(
		2026,
		time.September,
		28,
		14,
		24,
		13,
		0,
		time.UTC,
	)
	newWindowFirstAllowed := limiter.Allow(1, newWindowTime)

	if !previousFirstAllowed {
		t.Fatal("expected first request of previous window to be allowed")
	}
	if !previousSecondAllowed {
		t.Fatal("expected second request of previous window to be allowed")
	}
	if previousThirdAllowed {
		t.Fatal("expected third request of previous window to be rejected")
	}
	if !newWindowFirstAllowed {
		t.Fatal("expected first request of new window to be allowed")
	}
}

func TestAllow_UsersHaveIndependentLimits(t *testing.T) {
	limiter := NewFixedWindow(1)
	now := time.Date(
		2026,
		time.September,
		28,
		14,
		23,
		12,
		0,
		time.UTC,
	)

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
		t.Fatal("expected user two's first request to be allowed")
	}
}

func TestAllow_RequestAtWindowBoundaryIsAllowed(t *testing.T) {
	limiter := NewFixedWindow(1)
	endOfFirstWindow := time.Date(
		2026,
		time.September,
		28,
		14,
		23,
		59,
		999_999_999,
		time.UTC,
	)
	startOfSecondWindow := time.Date(
		2026,
		time.September,
		28,
		14,
		24,
		0,
		0,
		time.UTC,
	)

	firstWindowRequestAllowed := limiter.Allow(1, endOfFirstWindow)
	secondWindowRequestAllowed := limiter.Allow(1, startOfSecondWindow)

	if !firstWindowRequestAllowed {
		t.Fatal("expected request at end of first window to be allowed")
	}

	if !secondWindowRequestAllowed {
		t.Fatal("expected request at start of second window to be allowed")
	}
}

func TestAllow_ConcurrentRequestsRespectLimit(t *testing.T) {
	maxRequests := 10
	totalRequests := 100
	wg := sync.WaitGroup{}
	start := make(chan struct{})
	results := make(chan bool, totalRequests)

	now := time.Date(
		2026,
		time.September,
		28,
		14,
		24,
		12,
		0,
		time.UTC,
	)

	limiter := NewFixedWindow(maxRequests)

	for range totalRequests {
		wg.Add(1)
		go func() {
			defer wg.Done()

			<-start
			allowed := limiter.Allow(1, now)
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

	if allowedCount != maxRequests {
		t.Fatalf(
			"expected %d allowed requests, got %d",
			maxRequests,
			allowedCount,
		)
	}
}
