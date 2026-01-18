package vcs_provider

import (
	"context"
	"log"
	"net/http"
)

func RetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	if err != nil {
		log.Printf("connection error, will retry: %v", err)
		return true, nil
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		log.Printf("rate limited (status %d), will retry", resp.StatusCode)
		return true, nil
	}

	if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		log.Printf("rate limited (status 403), will retry")
		return true, nil
	}

	if resp.StatusCode >= 500 && resp.StatusCode != 501 {
		log.Printf("server error %d, will retry", resp.StatusCode)
		return true, nil
	}

	if resp.StatusCode >= 400 {
		log.Printf("client error %d - not retrying", resp.StatusCode)
		return false, nil
	}

	return false, nil
}
