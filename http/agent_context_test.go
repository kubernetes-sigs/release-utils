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

package http_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rhttp "sigs.k8s.io/release-utils/http"
)

// echoServer replies with the request method and path so the body of every
// request family is checkable.
func echoServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Method+" "+r.URL.Path) //nolint:gosec // test server echoing the request back
	}))
	t.Cleanup(server.Close)

	return server
}

// contextCalls exercises every context-taking method of the agent against a
// URL and folds its outcome into a single error, so the whole family can be
// driven from one table. Group methods report the error of every slot.
func contextCalls(agent *rhttp.Agent, u string) map[string]func(context.Context) error {
	data := []byte("payload")
	urls := []string{u, u}
	payloads := [][]byte{data, data}
	joinErrs := func(errs []error) error { return errors.Join(errs...) }
	closeBody := func(resp *http.Response, err error) error {
		if resp != nil {
			resp.Body.Close()
		}

		return err
	}
	closeBodies := func(resps []*http.Response, errs []error) error {
		for _, resp := range resps {
			if resp != nil {
				resp.Body.Close()
			}
		}

		return joinErrs(errs)
	}

	return map[string]func(context.Context) error{
		"GetContext": func(ctx context.Context) error {
			_, err := agent.GetContext(ctx, u)

			return err
		},
		"GetRequestContext": func(ctx context.Context) error {
			return closeBody(agent.GetRequestContext(ctx, u)) //nolint:bodyclose // closed by closeBody
		},
		"GetToWriterContext": func(ctx context.Context) error {
			return agent.GetToWriterContext(ctx, io.Discard, u)
		},
		"GetGroupContext": func(ctx context.Context) error {
			_, errs := agent.GetGroupContext(ctx, urls)

			return joinErrs(errs)
		},
		"GetRequestGroupContext": func(ctx context.Context) error {
			return closeBodies(agent.GetRequestGroupContext(ctx, urls)) //nolint:bodyclose // closed by closeBodies
		},
		"GetToWriterGroupContext": func(ctx context.Context) error {
			return joinErrs(agent.GetToWriterGroupContext(ctx, []io.Writer{io.Discard}, urls))
		},
		"PostContext": func(ctx context.Context) error {
			_, err := agent.PostContext(ctx, u, data)

			return err
		},
		"PostRequestContext": func(ctx context.Context) error {
			return closeBody(agent.PostRequestContext(ctx, u, data)) //nolint:bodyclose // closed by closeBody
		},
		"PostToWriterContext": func(ctx context.Context) error {
			return agent.PostToWriterContext(ctx, io.Discard, u, data)
		},
		"PostGroupContext": func(ctx context.Context) error {
			_, errs := agent.PostGroupContext(ctx, urls, payloads)

			return joinErrs(errs)
		},
		"PostRequestGroupContext": func(ctx context.Context) error {
			return closeBodies(agent.PostRequestGroupContext(ctx, urls, payloads)) //nolint:bodyclose // closed by closeBodies
		},
		"PostToWriterGroupContext": func(ctx context.Context) error {
			return joinErrs(agent.PostToWriterGroupContext(ctx, []io.Writer{io.Discard}, urls, payloads))
		},
		"HeadContext": func(ctx context.Context) error {
			_, err := agent.HeadContext(ctx, u)

			return err
		},
		"HeadRequestContext": func(ctx context.Context) error {
			return closeBody(agent.HeadRequestContext(ctx, u)) //nolint:bodyclose // closed by closeBody
		},
	}
}

// TestContextMethodsSucceed proves every context variant works end to end
// with a live context, so the cancellation test below is not passing for
// lack of a working request path.
func TestContextMethodsSucceed(t *testing.T) {
	t.Parallel()

	server := echoServer(t)
	agent := rhttp.NewAgent().WithRetries(0)

	for name, call := range contextCalls(agent, server.URL+"/ok") {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, call(t.Context()))
		})
	}
}

// TestContextMethodsHonorCancellation proves every context variant, group
// variants included, fails with context.Canceled when handed a cancelled
// context. Retries are left at their default so the test also proves a
// cancelled context is not retried.
func TestContextMethodsHonorCancellation(t *testing.T) {
	t.Parallel()

	server := echoServer(t)
	agent := rhttp.NewAgent().WithWaitTime(0)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	for name, call := range contextCalls(agent, server.URL+"/cancelled") {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := call(ctx)
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

// TestWrappersMatchContextVariants proves each legacy method returns the same
// content as its context variant with a background context.
func TestWrappersMatchContextVariants(t *testing.T) {
	t.Parallel()

	server := echoServer(t)
	agent := rhttp.NewAgent().WithRetries(0)
	u := server.URL + "/same"
	urls := []string{u, u}
	data := []byte("payload")
	payloads := [][]byte{data, data}

	t.Run("Get", func(t *testing.T) {
		t.Parallel()

		legacy, err := agent.Get(u)
		require.NoError(t, err)
		withCtx, err := agent.GetContext(t.Context(), u)
		require.NoError(t, err)
		assert.Equal(t, "GET /same", string(legacy))
		assert.Equal(t, legacy, withCtx)
	})

	t.Run("Post", func(t *testing.T) {
		t.Parallel()

		legacy, err := agent.Post(u, data)
		require.NoError(t, err)
		withCtx, err := agent.PostContext(t.Context(), u, data)
		require.NoError(t, err)
		assert.Equal(t, "POST /same", string(legacy))
		assert.Equal(t, legacy, withCtx)
	})

	t.Run("Head", func(t *testing.T) {
		t.Parallel()

		legacy, err := agent.Head(u)
		require.NoError(t, err)
		withCtx, err := agent.HeadContext(t.Context(), u)
		require.NoError(t, err)
		assert.Equal(t, legacy, withCtx)
	})

	t.Run("GetToWriter", func(t *testing.T) {
		t.Parallel()

		var legacy, withCtx bytes.Buffer

		require.NoError(t, agent.GetToWriter(&legacy, u))
		require.NoError(t, agent.GetToWriterContext(t.Context(), &withCtx, u))
		assert.Equal(t, "GET /same", legacy.String())
		assert.Equal(t, legacy.String(), withCtx.String())
	})

	t.Run("PostToWriter", func(t *testing.T) {
		t.Parallel()

		var legacy, withCtx bytes.Buffer

		require.NoError(t, agent.PostToWriter(&legacy, u, data))
		require.NoError(t, agent.PostToWriterContext(t.Context(), &withCtx, u, data))
		assert.Equal(t, "POST /same", legacy.String())
		assert.Equal(t, legacy.String(), withCtx.String())
	})

	t.Run("GetGroup", func(t *testing.T) {
		t.Parallel()

		legacy, errs := agent.GetGroup(urls)
		require.NoError(t, errors.Join(errs...))
		withCtx, errs := agent.GetGroupContext(t.Context(), urls)
		require.NoError(t, errors.Join(errs...))
		assert.Equal(t, legacy, withCtx)
	})

	t.Run("PostGroup", func(t *testing.T) {
		t.Parallel()

		legacy, errs := agent.PostGroup(urls, payloads)
		require.NoError(t, errors.Join(errs...))
		withCtx, errs := agent.PostGroupContext(t.Context(), urls, payloads)
		require.NoError(t, errors.Join(errs...))
		assert.Equal(t, legacy, withCtx)
	})
}
