package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"rate-limiter/slidingwindowcounter"
	"time"
)

type RateLimiter interface {
	Allow(userID int, now time.Time) bool
}

func main() {
	mux := http.NewServeMux()
	limiter := slidingwindowcounter.NewSlidingWindowCounter(2, time.Minute)

	mux.Handle(
		"GET /hello",
		RateLimitMiddleware(http.HandlerFunc(getHello), limiter),
	)

	mux.Handle(
		"GET /hello2",
		RateLimitMiddleware(HelloHandler{}, limiter),
	)

	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}

func getHello(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("got /hello request\n")
	io.WriteString(w, "Hello, HTTP!\n")
}

type HelloHandler struct {
}

func (h HelloHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	response := []byte("Hello world 22>!")
	w.Write(response)
}

func RateLimitMiddleware(next http.Handler, limiter RateLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(1, time.Now()) {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
