/*
Copyright 2025 The Kubernetes Authors.

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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rhttp "sigs.k8s.io/release-utils/http"
	"sigs.k8s.io/release-utils/http/httpfakes"
)

func TestGetRequest(t *testing.T) {
	for _, tc := range map[string]struct {
		prepare func(*httpfakes.FakeAgentImplementation)
		assert  func(*http.Response, error)
	}{
		"should succeed": {
			prepare: func(mock *httpfakes.FakeAgentImplementation) {
				mock.SendGetRequestReturns(&http.Response{StatusCode: http.StatusOK}, nil)
			},
			assert: func(response *http.Response, err error) {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, response.StatusCode)
			},
		},
		"should succeed on retry": {
			prepare: func(mock *httpfakes.FakeAgentImplementation) {
				mock.SendGetRequestReturnsOnCall(0, &http.Response{StatusCode: http.StatusInternalServerError}, nil)
				mock.SendGetRequestReturnsOnCall(1, &http.Response{StatusCode: http.StatusOK}, nil)
			},
			assert: func(response *http.Response, err error) {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, response.StatusCode)
			},
		},
		"should retry on internal server error": {
			prepare: func(mock *httpfakes.FakeAgentImplementation) {
				mock.SendGetRequestReturns(&http.Response{StatusCode: http.StatusInternalServerError}, nil)
			},
			assert: func(response *http.Response, err error) {
				require.Error(t, err)
				assert.NotNil(t, response)
			},
		},
		"should retry on too many requests": {
			prepare: func(mock *httpfakes.FakeAgentImplementation) {
				mock.SendGetRequestReturns(&http.Response{StatusCode: http.StatusTooManyRequests}, nil)
			},
			assert: func(response *http.Response, err error) {
				require.Error(t, err)
				assert.NotNil(t, response)
			},
		},
		"should retry on URL error": {
			prepare: func(mock *httpfakes.FakeAgentImplementation) {
				mock.SendGetRequestReturns(nil, &url.Error{Err: errors.New("test")})
			},
			assert: func(response *http.Response, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "test")
				assert.Nil(t, response)
			},
		},
	} {
		agent := rhttp.NewAgent().WithWaitTime(0)
		mock := &httpfakes.FakeAgentImplementation{}
		agent.SetImplementation(mock)

		if tc.prepare != nil {
			tc.prepare(mock)
		}

		//nolint:bodyclose // no need to close for mocked tests
		tc.assert(agent.GetRequest(""))
	}
}

func TestPostRequest(t *testing.T) {
	for _, tc := range map[string]struct {
		prepare func(*httpfakes.FakeAgentImplementation)
		assert  func(*http.Response, error)
	}{
		"should succeed": {
			prepare: func(mock *httpfakes.FakeAgentImplementation) {
				mock.SendPostRequestReturns(&http.Response{StatusCode: http.StatusOK}, nil)
			},
			assert: func(response *http.Response, err error) {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, response.StatusCode)
			},
		},
		"should succeed on retry": {
			prepare: func(mock *httpfakes.FakeAgentImplementation) {
				mock.SendPostRequestReturnsOnCall(0, &http.Response{StatusCode: http.StatusInternalServerError}, nil)
				mock.SendPostRequestReturnsOnCall(1, &http.Response{StatusCode: http.StatusOK}, nil)
			},
			assert: func(response *http.Response, err error) {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, response.StatusCode)
			},
		},
		"should retry on internal server error": {
			prepare: func(mock *httpfakes.FakeAgentImplementation) {
				mock.SendPostRequestReturns(&http.Response{StatusCode: http.StatusInternalServerError}, nil)
			},
			assert: func(response *http.Response, err error) {
				require.Error(t, err)
				assert.NotNil(t, response)
			},
		},
		"should retry on too many requests": {
			prepare: func(mock *httpfakes.FakeAgentImplementation) {
				mock.SendPostRequestReturns(&http.Response{StatusCode: http.StatusTooManyRequests}, nil)
			},
			assert: func(response *http.Response, err error) {
				require.Error(t, err)
				assert.NotNil(t, response)
			},
		},
		"should retry on URL error": {
			prepare: func(mock *httpfakes.FakeAgentImplementation) {
				mock.SendPostRequestReturns(nil, &url.Error{Err: errors.New("test")})
			},
			assert: func(response *http.Response, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "test")
				assert.Nil(t, response)
			},
		},
	} {
		agent := rhttp.NewAgent().WithWaitTime(0)
		mock := &httpfakes.FakeAgentImplementation{}
		agent.SetImplementation(mock)

		if tc.prepare != nil {
			tc.prepare(mock)
		}

		//nolint:bodyclose // no need to close for mocked tests
		tc.assert(agent.PostRequest("", nil))
	}
}

func TestWithClient(t *testing.T) {
	// Create a test server that returns a specific response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("custom-client-response")) //nolint:errcheck
	}))
	defer server.Close()

	customClient := &http.Client{}
	agent := rhttp.NewAgent().WithClient(customClient)

	// Verify the custom client is returned
	assert.Equal(t, customClient, agent.Client())

	// Make a request using the agent with the custom client
	response, err := agent.Get(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "custom-client-response", string(response))
}

// TestClientConfiguredByOptions verifies the agent's timeout lands on the
// client at configure time, whatever the order of the With calls. Client()
// itself no longer configures anything: the goroutines the group methods
// spawn call it concurrently, so it has to be a read.
func TestClientConfiguredByOptions(t *testing.T) {
	agent := rhttp.NewAgent().WithTimeout(9 * time.Second)
	assert.Equal(t, 9*time.Second, agent.Client().Timeout)

	// A custom client picks up the timeout already set...
	custom := &http.Client{}
	agent.WithClient(custom)
	assert.Equal(t, 9*time.Second, custom.Timeout)

	// ...and one set afterwards.
	agent.WithTimeout(11 * time.Second)
	assert.Equal(t, 11*time.Second, custom.Timeout)
}

// TestClientLeavesDefaultClientAlone guards against the agent adopting
// http.DefaultClient as its own, as it used to when no custom client was
// set: stamping the agent's timeout on it silently reconfigured every other
// user of the process-wide default.
func TestClientLeavesDefaultClientAlone(t *testing.T) {
	before := http.DefaultClient.Timeout

	agent := rhttp.NewAgent().WithTimeout(42 * time.Second)
	require.NotSame(t, http.DefaultClient, agent.Client())
	assert.Equal(t, before, http.DefaultClient.Timeout)
}

// TestGetGroupParallel fetches a group through a real server with real
// parallelism. Under the race detector it is the regression test for the
// data race the group methods used to have: every goroutine they spawned
// called Client(), which wrote the lazily-adopted client and its timeout
// on every call.
func TestGetGroupParallel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.URL.Path)) //nolint:errcheck,gosec // test server echoing the path back
	}))
	defer server.Close()

	urls := make([]string, 20)
	for i := range urls {
		urls[i] = fmt.Sprintf("%s/%d", server.URL, i)
	}

	agent := rhttp.NewAgent().WithMaxParallel(4)
	bodies, errs := agent.GetGroup(urls)
	require.Len(t, bodies, len(urls))

	for i := range urls {
		require.NoError(t, errs[i])
		assert.Equal(t, fmt.Sprintf("/%d", i), string(bodies[i]))
	}
}
