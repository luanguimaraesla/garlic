//go:build unit

package httpclient

import (
	"net/http"
	"testing"
	"time"
)

func TestResponse_RetryAfter(t *testing.T) {
	future := time.Now().Add(2 * time.Hour).UTC().Format(http.TimeFormat)
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)

	cases := []struct {
		name    string
		header  string
		want    time.Duration
		wantOK  bool
		checkFn func(*testing.T, time.Duration)
	}{
		{name: "missing", wantOK: false},
		{name: "invalid", header: "soon", wantOK: false},
		{name: "seconds", header: "120", want: 2 * time.Minute, wantOK: true},
		{name: "zero seconds", header: "0", want: 0, wantOK: true},
		{
			name:   "future date",
			header: future,
			wantOK: true,
			checkFn: func(t *testing.T, got time.Duration) {
				t.Helper()
				if got <= 0 || got > 2*time.Hour {
					t.Fatalf("RetryAfter duration = %v, want between 0 and 2h", got)
				}
			},
		},
		{name: "past date", header: past, want: 0, wantOK: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			header := http.Header{}
			if tc.header != "" {
				header.Set("Retry-After", tc.header)
			}
			resp := &Response{Response: textResponse(http.StatusTooManyRequests, "", header)}

			got, ok := resp.RetryAfter()
			if ok != tc.wantOK {
				t.Fatalf("RetryAfter ok = %v, want %v", ok, tc.wantOK)
			}
			if tc.checkFn != nil {
				tc.checkFn(t, got)
				return
			}
			if got != tc.want {
				t.Fatalf("RetryAfter duration = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResponse_RetryAfterNil(t *testing.T) {
	var resp *Response
	if got, ok := resp.RetryAfter(); ok || got != 0 {
		t.Fatalf("nil response RetryAfter = (%v, %v), want (0, false)", got, ok)
	}

	resp = &Response{}
	if got, ok := resp.RetryAfter(); ok || got != 0 {
		t.Fatalf("nil underlying response RetryAfter = (%v, %v), want (0, false)", got, ok)
	}
}
