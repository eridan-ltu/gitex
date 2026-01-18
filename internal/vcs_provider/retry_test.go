package vcs_provider

import (
	"context"
	"net/http"
	"testing"
)

func TestRetryPolicy(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		headers     map[string]string
		err         error
		wantRetry   bool
		wantErr     bool
		ctxCanceled bool
	}{
		{
			name:       "200 OK - no retry",
			statusCode: 200,
			wantRetry:  false,
		},
		{
			name:       "201 Created - no retry",
			statusCode: 201,
			wantRetry:  false,
		},
		{
			name:       "400 Bad Request - no retry",
			statusCode: 400,
			wantRetry:  false,
		},
		{
			name:       "401 Unauthorized - no retry",
			statusCode: 401,
			wantRetry:  false,
		},
		{
			name:       "403 Forbidden without rate limit - no retry",
			statusCode: 403,
			headers:    map[string]string{"X-RateLimit-Remaining": "100"},
			wantRetry:  false,
		},
		{
			name:       "403 Forbidden with rate limit exhausted - retry",
			statusCode: 403,
			headers:    map[string]string{"X-RateLimit-Remaining": "0"},
			wantRetry:  true,
		},
		{
			name:       "404 Not Found - no retry",
			statusCode: 404,
			wantRetry:  false,
		},
		{
			name:       "422 Unprocessable Entity - no retry",
			statusCode: 422,
			wantRetry:  false,
		},
		{
			name:       "429 Too Many Requests - retry",
			statusCode: 429,
			wantRetry:  true,
		},
		{
			name:       "500 Internal Server Error - retry",
			statusCode: 500,
			wantRetry:  true,
		},
		{
			name:       "501 Not Implemented - no retry",
			statusCode: 501,
			wantRetry:  false,
		},
		{
			name:       "502 Bad Gateway - retry",
			statusCode: 502,
			wantRetry:  true,
		},
		{
			name:       "503 Service Unavailable - retry",
			statusCode: 503,
			wantRetry:  true,
		},
		{
			name:      "connection error - retry",
			err:       context.DeadlineExceeded,
			wantRetry: true,
		},
		{
			name:        "context canceled - no retry, return error",
			ctxCanceled: true,
			wantRetry:   false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.ctxCanceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			var resp *http.Response
			if tt.err == nil && !tt.ctxCanceled {
				resp = &http.Response{
					StatusCode: tt.statusCode,
					Header:     make(http.Header),
				}
				for k, v := range tt.headers {
					resp.Header.Set(k, v)
				}
			}

			retry, err := RetryPolicy(ctx, resp, tt.err)

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if retry != tt.wantRetry {
				t.Errorf("retry = %v, want %v", retry, tt.wantRetry)
			}
		})
	}
}
