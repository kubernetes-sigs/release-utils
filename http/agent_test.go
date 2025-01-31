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
	"net/http"
	"net/url"
	"testing"

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
