/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingServer holds every request open until the test ends, so a call
// against it returns promptly only if the caller's context aborted it. The
// handler is released at cleanup rather than on the request's context: for a
// request with a body the server only notices a departed client once the
// body is read, and Close would otherwise wait on the handler forever.
func blockingServer(t *testing.T) *httptest.Server {
	t.Helper()

	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))

	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	return server
}

// TestDefaultImplementationHonorsContext proves each request the default
// implementation builds carries the caller's context: cancelling it aborts a
// request the server is holding open.
func TestDefaultImplementationHonorsContext(t *testing.T) {
	t.Parallel()

	impl := &defaultAgentImplementation{}

	for name, send := range map[string]func(context.Context, *http.Client, string) (*http.Response, error){
		"get":  impl.SendGetRequest,
		"head": impl.SendHeadRequest,
		"post": func(ctx context.Context, c *http.Client, u string) (*http.Response, error) {
			return impl.SendPostRequest(ctx, c, u, []byte("data"), "")
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := blockingServer(t)

			ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
			defer cancel()

			start := time.Now()
			resp, err := send(ctx, &http.Client{}, server.URL) //nolint:bodyclose // errors carry no body
			require.ErrorIs(t, err, context.DeadlineExceeded)
			assert.Nil(t, resp)
			assert.Less(t, time.Since(start), 5*time.Second, "request must abort when the context ends")
		})
	}
}

// TestDefaultImplementationPostContentType guards the behavior client.Post
// used to provide: the content type reaches the server, defaulting when empty.
func TestDefaultImplementationPostContentType(t *testing.T) {
	t.Parallel()

	var got atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Store(r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	impl := &defaultAgentImplementation{}

	for contentType, expected := range map[string]string{
		"application/json": "application/json",
		"":                 defaultPostContentType,
	} {
		resp, err := impl.SendPostRequest(t.Context(), &http.Client{}, server.URL, []byte("{}"), contentType)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, expected, got.Load())
	}
}

// TestRetryStopsWhenContextEnds proves the retry loops neither attempt again
// nor wait out a backoff once the context is done, and that the context error
// surfaces through errors.Is. The wait time is far longer than the test
// budget, so finishing at all means the sleep was interrupted.
func TestRetryStopsWhenContextEnds(t *testing.T) {
	t.Parallel()

	newAgent := func() *Agent {
		return NewAgent().WithRetries(5).WithWaitTime(time.Hour).WithMaxWaitTime(time.Hour)
	}
	transient := &url.Error{Op: "Get", URL: "http://example.com/", Err: errors.New("connection refused")}

	t.Run("retryRequest", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())

		var attempts atomic.Int32

		_, err := newAgent().retryRequest(ctx, func() (*http.Response, error) { //nolint:bodyclose // errors carry no body
			attempts.Add(1)
			cancel()

			return nil, transient
		})
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, int32(1), attempts.Load())
	})

	t.Run("retryOperation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())

		var attempts atomic.Int32

		err := newAgent().retryOperation(ctx, func() error {
			attempts.Add(1)
			cancel()

			return transient
		})
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, int32(1), attempts.Load())
	})

	t.Run("already cancelled", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		var attempts atomic.Int32

		err := newAgent().retryOperation(ctx, func() error {
			attempts.Add(1)

			return transient
		})
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, int32(0), attempts.Load(), "a cancelled context must not start a first attempt")
	})
}

// TestHeadRequestBackoffHonorsContext proves the hand-rolled HEAD retry loop
// abandons its backoff sleep when the context ends. The first backoff is two
// seconds; the context ends well before that.
func TestHeadRequestBackoffHonorsContext(t *testing.T) {
	t.Parallel()

	agent := NewAgent().WithRetries(3)
	agent.SetImplementation(failingImpl{err: errors.New("boom")})

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := agent.headRequest(ctx, "http://example.com/") //nolint:bodyclose // errors carry no body
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), time.Second, "backoff must be interrupted by the context")
}

// failingImpl is an AgentImplementation whose every request fails.
type failingImpl struct{ err error }

func (f failingImpl) SendPostRequest(context.Context, *http.Client, string, []byte, string) (*http.Response, error) {
	return nil, f.err
}

func (f failingImpl) SendGetRequest(context.Context, *http.Client, string) (*http.Response, error) {
	return nil, f.err
}

func (f failingImpl) SendHeadRequest(context.Context, *http.Client, string) (*http.Response, error) {
	return nil, f.err
}
