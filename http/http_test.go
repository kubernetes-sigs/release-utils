/*
Copyright 2020 The Kubernetes Authors.

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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	khttp "sigs.k8s.io/release-utils/http"
	"sigs.k8s.io/release-utils/http/httpfakes"
)

func TestGetURLResponseSuccess(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_, err := io.WriteString(w, "")
			if err != nil {
				t.Fail()
			}
		}))
	defer server.Close()

	// When
	actual, err := khttp.GetURLResponse(server.URL, false)

	// Then
	require.NoError(t, err)
	require.Empty(t, actual)
}

func TestGetURLResponseSuccessTrimmed(t *testing.T) {
	// Given
	const expected = "     some test     "
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_, err := io.WriteString(w, expected)
			if err != nil {
				t.Fail()
			}
		}))
	defer server.Close()

	// When
	actual, err := khttp.GetURLResponse(server.URL, true)

	// Then
	require.NoError(t, err)
	require.Equal(t, strings.TrimSpace(expected), actual)
}

func TestGetURLResponseFailedStatus(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
	defer server.Close()

	// When
	_, err := khttp.GetURLResponse(server.URL, true)

	// Then
	require.Error(t, err)
}

func NewTestAgent() *khttp.Agent {
	agent := khttp.NewAgent()
	agent.SetImplementation(&httpfakes.FakeAgentImplementation{})
	return agent
}

func TestAgentPost(t *testing.T) {
	t.Parallel()
	agent := NewTestAgent().WithRetries(0)
	resp := getTestResponse()
	defer resp.Body.Close()

	// First simulate a successful request
	fake := &httpfakes.FakeAgentImplementation{}
	fake.SendPostRequestReturns(resp, nil)

	agent.SetImplementation(fake)
	body, err := agent.Post("http://www.example.com/", []byte("Test string"))
	require.NoError(t, err)
	require.Equal(t, body, []byte("hello sig-release!"))

	// Now check error is handled
	fake.SendPostRequestReturns(resp, errors.New("HTTP Post error"))
	agent.SetImplementation(fake)
	_, err = agent.Post("http://www.example.com/", []byte("Test string"))
	require.Error(t, err)
}

func TestAgentGet(t *testing.T) {
	t.Parallel()
	agent := NewTestAgent().WithRetries(0)

	for _, tc := range []struct {
		name     string
		mustErr  bool
		expected []byte
		prepare  func(*httpfakes.FakeAgentImplementation)
	}{
		{
			"no-error",
			false,
			[]byte("hello sig-release!"),
			func(fai *httpfakes.FakeAgentImplementation) {
				t.Helper()
				resp := getTestResponse()
				defer resp.Body.Close()
				fai.SendGetRequestReturns(resp, nil)
			},
		}, {
			"error",
			true,
			nil,
			func(fai *httpfakes.FakeAgentImplementation) {
				t.Helper()
				fai.SendGetRequestReturns(nil, errors.New("HTTP Post error"))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := &httpfakes.FakeAgentImplementation{}
			tc.prepare(fake)
			agent.SetImplementation(fake)
			b, err := agent.Get("http://www.example.com/")
			if tc.mustErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, b)
		})
	}
}

func TestAgentGetToWriter(t *testing.T) {
	agent := NewTestAgent()
	for _, tc := range []struct {
		n       string
		prepare func(*httpfakes.FakeAgentImplementation, *http.Response)
		mustErr bool
	}{
		{
			n: "success",
			prepare: func(fake *httpfakes.FakeAgentImplementation, resp *http.Response) {
				fake.SendGetRequestReturns(resp, nil)
			},
		},
		{
			n: "fail",
			prepare: func(fake *httpfakes.FakeAgentImplementation, resp *http.Response) {
				fake.SendGetRequestReturns(resp, errors.New("HTTP Post error"))
			},
			mustErr: true,
		},
	} {
		t.Run(tc.n, func(t *testing.T) {
			// First simulate a successful request
			fake := &httpfakes.FakeAgentImplementation{}
			resp := getTestResponse()
			defer resp.Body.Close()
			tc.prepare(fake, resp)
			var buf bytes.Buffer

			agent.SetImplementation(fake)
			err := agent.GetToWriter(&buf, "http://www.example.com/")
			if tc.mustErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, buf.Bytes(), []byte("hello sig-release!"))
		})
	}
}

func TestAgentHead(t *testing.T) {
	t.Parallel()
	agent := NewTestAgent().WithRetries(0)

	resp := getTestResponse()
	defer resp.Body.Close()

	// First simulate a successful request
	fake := &httpfakes.FakeAgentImplementation{}
	fake.SendHeadRequestReturns(resp, nil)

	agent.SetImplementation(fake)
	b, err := agent.Head("http://www.example.com/")
	require.NoError(t, err)
	require.Equal(t, b, []byte("hello sig-release!"))

	// Now check error is handled
	fake.SendHeadRequestReturns(resp, errors.New("HTTP Head error"))
	agent.SetImplementation(fake)
	_, err = agent.Head("http://www.example.com/")
	require.Error(t, err)
}

func getTestResponse() *http.Response {
	return &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Body:          io.NopCloser(bytes.NewReader([]byte("hello sig-release!"))),
		ContentLength: 18,
		Close:         true,
		Request:       &http.Request{},
	}
}

func TestAgentPostToWriter(t *testing.T) {
	for _, tc := range []struct {
		n       string
		prepare func(*httpfakes.FakeAgentImplementation, *http.Response)
		mustErr bool
	}{
		{
			n: "success",
			prepare: func(fake *httpfakes.FakeAgentImplementation, resp *http.Response) {
				fake.SendPostRequestReturns(resp, nil)
			},
		},
		{
			n: "fail",
			prepare: func(fake *httpfakes.FakeAgentImplementation, resp *http.Response) {
				fake.SendPostRequestReturns(resp, errors.New("HTTP Post error"))
			},
			mustErr: true,
		},
	} {
		t.Run(tc.n, func(t *testing.T) {
			agent := NewTestAgent()
			// First simulate a successful request
			fake := &httpfakes.FakeAgentImplementation{}
			resp := getTestResponse()
			defer resp.Body.Close()
			tc.prepare(fake, resp)
			var buf bytes.Buffer
			agent.SetImplementation(fake)
			err := agent.PostToWriter(&buf, "http://www.example.com/", []byte{})
			if tc.mustErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, buf.Bytes(), []byte("hello sig-release!"))
		})
	}
}

func TestAgentOptions(t *testing.T) {
	agent := NewTestAgent()
	fake := &httpfakes.FakeAgentImplementation{}
	resp := &http.Response{
		Status:        "Fake not found",
		StatusCode:    http.StatusNotFound,
		Body:          io.NopCloser(bytes.NewReader([]byte("hello sig-release!"))),
		ContentLength: 18,
		Close:         true,
		Request:       &http.Request{},
	}
	defer resp.Body.Close()

	fake.SendGetRequestReturns(resp, nil)
	agent.SetImplementation(fake)

	// Test FailOnHTTPError
	// First we fail on server errors
	_, err := agent.WithFailOnHTTPError(true).Get("http://example.com/")
	require.Error(t, err)

	// Then we just note them and do not fail
	_, err = agent.WithFailOnHTTPError(false).Get("http://example.com/")
	require.NoError(t, err)
}

// closeHTTPResponseGroup is an internal func that closes the response bodies.
func closeHTTPResponseGroup(resps []*http.Response) {
	for i := range resps {
		if resps[i] == nil {
			continue
		}
		resps[i].Body.Close()
	}
}

func TestAgentGroupGetRequest(t *testing.T) {
	fake := &httpfakes.FakeAgentImplementation{}
	fakeUrls := []string{"http://www/1", "http://www/2", "http://www/3"}
	fake.SendGetRequestCalls(func(_ *http.Client, s string) (*http.Response, error) {
		switch s {
		case fakeUrls[0]:
			return &http.Response{
				Status:        "Fake OK",
				StatusCode:    http.StatusOK,
				Body:          io.NopCloser(bytes.NewReader([]byte("hello sig-release!"))),
				ContentLength: 18,
				Close:         true,
				Request:       &http.Request{},
			}, nil
		case fakeUrls[1]:
			return &http.Response{
				Status:        "Fake not found",
				StatusCode:    http.StatusNotFound,
				Body:          io.NopCloser(bytes.NewReader([]byte("hello sig-release!"))),
				ContentLength: 18,
				Close:         true,
				Request:       &http.Request{},
			}, nil
		case fakeUrls[2]:
			return nil, errors.New("malformed url")
		}
		return nil, nil
	})

	for _, tc := range []struct {
		name    string
		workers int
	}{
		{"no-parallelism", 1}, {"one-per-request", 3}, {"spare-workers", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// No retries as the errors are synthetic
			agent := NewTestAgent().WithRetries(0).WithFailOnHTTPError(false).WithMaxParallel(tc.workers)
			agent.SetImplementation(fake)

			//nolint: bodyclose // The next line closes them
			resps, errs := agent.GetRequestGroup(fakeUrls)
			defer closeHTTPResponseGroup(resps)

			require.Len(t, resps, 3)
			require.Len(t, errs, 3)

			require.NoError(t, errs[0])
			require.NoError(t, errs[1])
			require.Error(t, errs[2])

			require.Equal(t, http.StatusOK, resps[0].StatusCode)
			require.Equal(t, http.StatusNotFound, resps[1].StatusCode)
			require.Nil(t, resps[2])
		})
	}
}

func TestAgentPostRequestGroup(t *testing.T) {
	t.Parallel()
	fake := &httpfakes.FakeAgentImplementation{}
	errorURL := "fake:error"
	httpErrorURL := "fake:httpError"
	noErrorURL := "fake:ok"

	fake.SendPostRequestCalls(func(_ *http.Client, s string, _ []byte, _ string) (*http.Response, error) {
		switch s {
		case noErrorURL:
			return &http.Response{
				Status:        "Fake OK",
				StatusCode:    http.StatusOK,
				Body:          io.NopCloser(bytes.NewReader([]byte("hello sig-release!"))),
				ContentLength: 18,
				Close:         true,
				Request:       &http.Request{},
			}, nil
		case httpErrorURL:
			return &http.Response{
				Status:        "Fake not found",
				StatusCode:    http.StatusNotFound,
				Body:          io.NopCloser(bytes.NewReader([]byte("hello sig-release!"))),
				ContentLength: 18,
				Close:         true,
				Request:       &http.Request{},
			}, fmt.Errorf("HTTP error %d for %s", http.StatusNotFound, s)
		case errorURL:
			return nil, errors.New("malformed url")
		}
		return nil, nil
	})

	for _, tc := range []struct {
		name     string
		workers  int
		mustErr  bool
		urls     []string
		postData [][]byte
	}{
		{"no-parallelism", 1, false, []string{noErrorURL, noErrorURL, noErrorURL}, make([][]byte, 3)},
		{"one-per-request", 3, false, []string{noErrorURL, noErrorURL, noErrorURL}, make([][]byte, 3)},
		{"spare-workers", 5, false, []string{noErrorURL, noErrorURL, noErrorURL}, make([][]byte, 3)},
		{"uneven-postdata", 5, true, []string{noErrorURL, noErrorURL, noErrorURL}, make([][]byte, 2)},
		{"uneven-postdata2", 5, true, []string{noErrorURL, noErrorURL, noErrorURL}, make([][]byte, 4)},
		{"http-error", 5, true, []string{noErrorURL, httpErrorURL, noErrorURL}, make([][]byte, 3)},
		{"software-error", 5, true, []string{noErrorURL, errorURL, noErrorURL}, make([][]byte, 3)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// No retries as the errors are synthetic
			agent := NewTestAgent().WithRetries(0).WithFailOnHTTPError(false).WithMaxParallel(tc.workers)
			agent.SetImplementation(fake)

			//nolint: bodyclose
			resps, errs := agent.PostRequestGroup(tc.urls, tc.postData)
			closeHTTPResponseGroup(resps)

			// If urls and postdata don't all errors should be errors
			if len(tc.urls) != len(tc.postData) {
				for i := range errs {
					require.Error(t, errs[i])
				}
				return
			}

			// Check for at least on error
			if tc.mustErr {
				require.Error(t, errors.Join(errs...))
			} else {
				require.NoError(t, errors.Join(errs...))
			}

			require.Len(t, resps, len(tc.urls))
			require.Len(t, errs, len(tc.urls))

			for i := range tc.urls {
				switch tc.urls[i] {
				case noErrorURL:
					require.NoError(t, errs[i])
					require.NotNil(t, resps[i])
					require.Equal(t, http.StatusOK, resps[i].StatusCode)
				case httpErrorURL:
					require.Error(t, errs[i])
					require.NotNil(t, resps[i])
					require.Equal(t, http.StatusNotFound, resps[i].StatusCode)
				case errorURL:
					require.Error(t, errs[i])
				}
			}
		})
	}
}
