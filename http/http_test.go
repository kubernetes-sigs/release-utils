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
	"io"
	"io/ioutil"
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
		func(w http.ResponseWriter, r *http.Request) {
			_, err := io.WriteString(w, "")
			require.Nil(t, err)
		}))
	defer server.Close()

	// When
	actual, err := khttp.GetURLResponse(server.URL, false)

	// Then
	require.Nil(t, err)
	require.Empty(t, actual)
}

func TestGetURLResponseSuccessTrimmed(t *testing.T) {
	// Given
	const expected = "     some test     "
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_, err := io.WriteString(w, expected)
			require.Nil(t, err)
		}))
	defer server.Close()

	// When
	actual, err := khttp.GetURLResponse(server.URL, true)

	// Then
	require.Nil(t, err)
	require.Equal(t, strings.TrimSpace(expected), actual)
}

func TestGetURLResponseFailedStatus(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
	defer server.Close()

	// When
	_, err := khttp.GetURLResponse(server.URL, true)

	// Then
	require.NotNil(t, err)
}

func NewTestAgent() *khttp.Agent {
	agent := khttp.NewAgent()
	agent.SetImplementation(&httpfakes.FakeAgentImplementation{})
	return agent
}

func TestAgentPost(t *testing.T) {
	agent := NewTestAgent()

	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Body:          ioutil.NopCloser(bytes.NewReader([]byte("hello sig-release!"))),
		ContentLength: 18,
		Close:         true,
		Request:       &http.Request{},
	}
	defer resp.Body.Close()

	// First simulate a successful request
	fake := &httpfakes.FakeAgentImplementation{}
	fake.SendPostRequestReturns(resp, nil)

	agent.SetImplementation(fake)
	body, err := agent.Post("http://www.example.com/", []byte("Test string"))
	require.Nil(t, err)
	require.Equal(t, body, []byte("hello sig-release!"))

	// Now check error is handled
	fake.SendPostRequestReturns(resp, errors.New("HTTP Post error"))
	agent.SetImplementation(fake)
	_, err = agent.Post("http://www.example.com/", []byte("Test string"))
	require.NotNil(t, err)
}

func TestAgentGet(t *testing.T) {
	agent := NewTestAgent()

	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Body:          ioutil.NopCloser(bytes.NewReader([]byte("hello sig-release!"))),
		ContentLength: 18,
		Close:         true,
		Request:       &http.Request{},
	}
	defer resp.Body.Close()

	// First simulate a successful request
	fake := &httpfakes.FakeAgentImplementation{}
	fake.SendGetRequestReturns(resp, nil)

	agent.SetImplementation(fake)
	b, err := agent.Get("http://www.example.com/")
	require.Nil(t, err)
	require.Equal(t, b, []byte("hello sig-release!"))

	// Now check error is handled
	fake.SendGetRequestReturns(resp, errors.New("HTTP Post error"))
	agent.SetImplementation(fake)
	_, err = agent.Get("http://www.example.com/")
	require.NotNil(t, err)
}

func TestAgentOptions(t *testing.T) {
	agent := NewTestAgent()
	fake := &httpfakes.FakeAgentImplementation{}
	resp := &http.Response{
		Status:        "Fake not found",
		StatusCode:    404,
		Body:          ioutil.NopCloser(bytes.NewReader([]byte("hello sig-release!"))),
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
	require.NotNil(t, err)

	// Then we just note them and do not fail
	_, err = agent.WithFailOnHTTPError(false).Get("http://example.com/")
	require.Nil(t, err)
}
