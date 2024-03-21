package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/time/rate"
)

var (
	URL = envOr("URL", "http://localhost:8080")
	RPS = mustFloat(envOr("RPS", "1"))
)

func envOr(k, v string) string {
	if v2 := os.Getenv(k); v2 != "" {
		return v2
	}
	return v
}

func mustFloat(s string) float64 {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	if v, err := strconv.ParseInt(s, 10, 32); err == nil {
		return float64(v)
	}
	panic(fmt.Errorf("%s is not a float", s))
}

func main() {
	u, err := url.Parse(URL)
	if err != nil {
		panic(err)
	}
	log.Println(u)

	ctx, can := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer can()

	go func() {
		for ctx.Err() == nil {
			time.Sleep(time.Second)
			http.ListenAndServe(":48081", http.HandlerFunc(http.NotFound))
		}
	}()

	limiter := rate.NewLimiter(rate.Limit(RPS), 1)
	for limiter.Wait(ctx) == nil {
		func() {
			req, err := http.NewRequest(http.MethodGet, u.String(), nil)
			if err != nil {
				panic(err)
			}
			req = req.WithContext(ctx)

			c := &http.Client{
				Timeout: time.Minute,
				CheckRedirect: func(*http.Request, []*http.Request) error {
					return http.ErrUseLastResponse
				},
				Transport: &http.Transport{DisableKeepAlives: true},
			}
			resp, err := c.Do(req)
			if err != nil {
				log.Printf("failed to Do %s %s: %v", req.Method, req.URL.String(), err)
				return
			}
			defer resp.Body.Close()

			b, _ := io.ReadAll(resp.Body)
			log.Printf("(%d) %s", resp.StatusCode, b)
		}()
	}
}
