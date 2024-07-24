/*
Copyright 2021 The Kubernetes Authors.

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
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/nozzle/throttler"
	"github.com/sirupsen/logrus"
)

const (
	defaultPostContentType = "application/octet-stream"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//go:generate /usr/bin/env bash -c "cat ../scripts/boilerplate/boilerplate.generatego.txt httpfakes/fake_agent_implementation.go > httpfakes/_fake_agent_implementation.go && mv httpfakes/_fake_agent_implementation.go httpfakes/fake_agent_implementation.go"

// Agent is an http agent.
type Agent struct {
	options *agentOptions
	AgentImplementation
}

// AgentImplementation is the actual implementation of the http calls
//
//counterfeiter:generate . AgentImplementation
type AgentImplementation interface {
	SendPostRequest(*http.Client, string, []byte, string) (*http.Response, error)
	SendGetRequest(*http.Client, string) (*http.Response, error)
	SendHeadRequest(*http.Client, string) (*http.Response, error)
}

type defaultAgentImplementation struct{}

// agentOptions has the configurable bits of the agent.
type agentOptions struct {
	FailOnHTTPError bool          // Set to true to fail on HTTP Status > 299
	Retries         uint          // Number of times to retry when errors happen
	Timeout         time.Duration // Timeout when fetching URLs
	MaxWaitTime     time.Duration // Max waiting time when backing off
	PostContentType string        // Content type to send when posting data
	MaxParallel     uint          // Maximum number of parallel requests when requesting groups
}

// String returns a string representation of the options.
func (ao *agentOptions) String() string {
	return fmt.Sprintf(
		"HTTP.Agent options: Timeout: %d - Retries: %d - FailOnHTTPError: %+v",
		ao.Timeout, ao.Retries, ao.FailOnHTTPError,
	)
}

var defaultAgentOptions = &agentOptions{
	FailOnHTTPError: true,
	Retries:         3,
	Timeout:         3 * time.Second,
	MaxWaitTime:     60 * time.Second,
	PostContentType: defaultPostContentType,
	MaxParallel:     5,
}

// NewAgent return a new agent with default options.
func NewAgent() *Agent {
	return &Agent{
		AgentImplementation: &defaultAgentImplementation{},
		options:             defaultAgentOptions,
	}
}

// SetImplementation sets the agent implementation.
func (a *Agent) SetImplementation(impl AgentImplementation) {
	a.AgentImplementation = impl
}

// WithTimeout sets the agent timeout.
func (a *Agent) WithTimeout(timeout time.Duration) *Agent {
	a.options.Timeout = timeout
	return a
}

// WithRetries sets the number of times we'll attempt to fetch the URL.
func (a *Agent) WithRetries(retries uint) *Agent {
	a.options.Retries = retries
	return a
}

// WithFailOnHTTPError determines if the agent fails on HTTP errors (HTTP status not in 200s).
func (a *Agent) WithFailOnHTTPError(flag bool) *Agent {
	a.options.FailOnHTTPError = flag
	return a
}

// WithMaxParallel controls how many requests we do when fetching groups.
func (a *Agent) WithMaxParallel(workers int) *Agent {
	a.options.MaxParallel = uint(workers)
	return a
}

// Client return an net/http client preconfigured with the agent options.
func (a *Agent) Client() *http.Client {
	return &http.Client{
		Timeout: a.options.Timeout,
	}
}

// Get returns the body a a GET request.
func (a *Agent) Get(url string) (content []byte, err error) {
	request, err := a.GetRequest(url)
	if err != nil {
		return nil, fmt.Errorf("getting GET request: %w", err)
	}
	defer request.Body.Close()

	return a.readResponseToByteArray(request)
}

// GetRequest sends a GET request to a URL and returns the request and response.
func (a *Agent) GetRequest(url string) (response *http.Response, err error) {
	logrus.Debugf("Sending GET request to %s", url)
	try := 0
	for {
		response, err = a.AgentImplementation.SendGetRequest(a.Client(), url)
		try++
		if err == nil || try >= int(a.options.Retries) {
			return response, err
		}
		// Do exponential backoff...
		waitTime := math.Pow(2, float64(try))
		//  ... but wait no more than 1 min
		if waitTime > 60 {
			waitTime = a.options.MaxWaitTime.Seconds()
		}
		logrus.Errorf(
			"Error getting URL (will retry %d more times in %.0f secs): %s",
			int(a.options.Retries)-try, waitTime, err.Error(),
		)
		time.Sleep(time.Duration(waitTime) * time.Second)
	}
}

// Post returns the body of a POST request.
func (a *Agent) Post(url string, postData []byte) (content []byte, err error) {
	response, err := a.PostRequest(url, postData)
	if err != nil {
		return nil, fmt.Errorf("getting post request: %w", err)
	}
	defer response.Body.Close()

	return a.readResponseToByteArray(response)
}

// PostRequest sends the postData in a POST request to a URL and returns the request object.
func (a *Agent) PostRequest(url string, postData []byte) (response *http.Response, err error) {
	logrus.Debugf("Sending POST request to %s", url)
	try := 0
	for {
		response, err = a.AgentImplementation.SendPostRequest(a.Client(), url, postData, a.options.PostContentType)
		try++
		if err == nil || try >= int(a.options.Retries) {
			return response, err
		}
		// Do exponential backoff...
		waitTime := math.Pow(2, float64(try))
		//  ... but wait no more than 1 min
		if waitTime > 60 {
			waitTime = a.options.MaxWaitTime.Seconds()
		}
		logrus.Errorf(
			"Error getting URL (will retry %d more times in %.0f secs): %s",
			int(a.options.Retries)-try, waitTime, err.Error(),
		)
		time.Sleep(time.Duration(waitTime) * time.Second)
	}
}

// Head returns the body of a HEAD request.
func (a *Agent) Head(url string) (content []byte, err error) {
	response, err := a.HeadRequest(url)
	if err != nil {
		return nil, fmt.Errorf("getting head request: %w", err)
	}
	defer response.Body.Close()

	return a.readResponseToByteArray(response)
}

// HeadRequest sends a HEAD request to a URL and returns the request and response.
func (a *Agent) HeadRequest(url string) (response *http.Response, err error) {
	logrus.Debugf("Sending HEAD request to %s", url)
	try := 0
	for {
		response, err = a.AgentImplementation.SendHeadRequest(a.Client(), url)
		try++
		if err == nil || try >= int(a.options.Retries) {
			return response, err
		}
		// Do exponential backoff...
		waitTime := math.Pow(2, float64(try))
		//  ... but wait no more than 1 min
		if waitTime > 60 {
			waitTime = a.options.MaxWaitTime.Seconds()
		}
		logrus.Errorf(
			"Error getting URL (will retry %d more times in %.0f secs): %s",
			int(a.options.Retries)-try, waitTime, err.Error(),
		)
		time.Sleep(time.Duration(waitTime) * time.Second)
	}
}

// SendPostRequest sends the actual HTTP post to the server.
func (impl *defaultAgentImplementation) SendPostRequest(
	client *http.Client, url string, postData []byte, contentType string,
) (response *http.Response, err error) {
	if contentType == "" {
		contentType = defaultPostContentType
	}
	response, err = client.Post(url, contentType, bytes.NewBuffer(postData))
	if err != nil {
		return response, fmt.Errorf("posting data to %s: %w", url, err)
	}
	return response, nil
}

// SendGetRequest performs the actual request.
func (impl *defaultAgentImplementation) SendGetRequest(client *http.Client, url string) (
	response *http.Response, err error,
) {
	response, err = client.Get(url)
	if err != nil {
		return response, fmt.Errorf("getting %s: %w", url, err)
	}

	return response, nil
}

// SendHeadRequest performs the actual request.
func (impl *defaultAgentImplementation) SendHeadRequest(client *http.Client, url string) (
	response *http.Response, err error,
) {
	response, err = client.Head(url)
	if err != nil {
		return response, fmt.Errorf("sending head request %s: %w", url, err)
	}

	return response, nil
}

// readResponseToByteArray returns the contents of an http response as a byte array.
func (a *Agent) readResponseToByteArray(response *http.Response) ([]byte, error) {
	var b bytes.Buffer
	if err := a.readResponse(response, &b); err != nil {
		return nil, fmt.Errorf("reading array buffer: %w", err)
	}
	return b.Bytes(), nil
}

// readResponse reads and interprets the response to an HTTP request to an io.Writer.
// If the response status is < 200 or >= 300 and FailOnHTTPError is set, the function
// will return an error.
//
// This function will close the response body reader.
func (a *Agent) readResponse(response *http.Response, w io.Writer) (err error) {
	// Read the response body
	defer response.Body.Close()
	if _, err := io.Copy(w, response.Body); err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	// Check the https response code
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if a.options.FailOnHTTPError {
			return fmt.Errorf(
				"HTTP error %s for %s", response.Status, response.Request.URL,
			)
		}
		logrus.Warnf("Got HTTP error but FailOnHTTPError not set: %s", response.Status)
	}
	return err
}

// GetToWriter sends a get request and writes the response to an io.Writer.
func (a *Agent) GetToWriter(w io.Writer, url string) error {
	resp, err := a.AgentImplementation.SendGetRequest(a.Client(), url)
	if err != nil {
		return fmt.Errorf("sending GET request: %w", err)
	}

	return a.readResponse(resp, w)
}

// PostToWriter sends a request to a url and writes the response to an io.Writer.
func (a *Agent) PostToWriter(w io.Writer, url string, postData []byte) error {
	resp, err := a.AgentImplementation.SendPostRequest(a.Client(), url, postData, a.options.PostContentType)
	if err != nil {
		return fmt.Errorf("sending POST request: %w", err)
	}
	return a.readResponse(resp, w)
}

// GetRequestGroup behaves like agent.SendGetRequest() but takes a group of URLs
// and performs the requests in parallel. The number of simultaneous requests is
// controlled by options.MaxParallel.
func (a *Agent) GetRequestGroup(urls []string) ([]*http.Response, []error) {
	t := throttler.New(int(a.options.MaxParallel), len(urls))
	ret := make([]*http.Response, len(urls))
	errs := make([]error, len(urls))
	m := sync.Mutex{}
	for i := range urls {
		i := i
		go func(url string) {
			//nolint: bodyclose // We don't close here as we're returning the response
			resp, err := a.AgentImplementation.SendGetRequest(a.Client(), url)

			m.Lock()
			ret[i] = resp
			errs[i] = err
			m.Unlock()

			t.Done(err)
		}(urls[i])
		t.Throttle()
	}

	return ret, errs
}

// PostRequestGroup behaves like agent.Post() but takes a group of URLs and performs the
// requests in parallel. The number of simultaneous requests is controlled by
// options.MaxParallel.
//
// The list of URLs and postData byte arrays are required to be of equal length.
// If postData has less elements than the URL list, the function will exit early,
// failing all requests.
func (a *Agent) PostRequestGroup(urls []string, postData [][]byte) ([]*http.Response, []error) {
	ret := make([]*http.Response, len(urls))
	errs := make([]error, len(urls))
	// URLs and postData arrays must be equal in length. If not exit now.
	if len(postData) != len(urls) {
		err := errors.New("unable to perform requests, same number URLs and POST payloads required")
		for i := 0; i < len(urls); i++ {
			errs[i] = err
		}
		return ret, errs
	}

	t := throttler.New(int(a.options.MaxParallel), len(urls))
	m := sync.Mutex{}
	for i := range urls {
		i := i
		go func(url string, pdata []byte) {
			//nolint: bodyclose // We don't close here as we're returning the raw response
			resp, err := a.AgentImplementation.SendPostRequest(
				a.Client(), url, pdata, a.options.PostContentType,
			)

			m.Lock()
			ret[i] = resp
			errs[i] = err
			m.Unlock()
			t.Done(err)
		}(urls[i], postData[i])
		t.Throttle()
	}

	return ret, errs
}

// PostGroup behaves just as Post() but takes a group of URLs and performs
// the requests in parallel. The number of simultaneous requests is controlled by
// options.MaxParallel.
//
// The list of URLs and postData byte arrays are expected to be of equal length.
// If postData has less elements than the url list, those urls without a corresponding
// postData array will return an error.
func (a *Agent) PostGroup(urls []string, postData [][]byte) ([][]byte, []error) {
	//nolint: bodyclose // Next line closes them
	resps, errs := a.PostRequestGroup(urls, postData)
	defer closeHTTPResponseGroup(resps)

	c := make([][]byte, len(urls))
	for i, r := range resps {
		if r != nil {
			d, err := a.readResponseToByteArray(r)
			if err != nil {
				errs[i] = fmt.Errorf("reading group response #%d: %w", i, err)
				continue
			}
			c[i] = d
		}
	}
	return c, errs
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

// PostToWriterGroup behaves just as PostToWriter() but takes a group of URLs
// and performs the requests in parallel. The number of simultaneous requests
// is controlled by options.MaxParallel.
//
// The list of URLs and postData byte arrays are expected to be of equal length.
// If postData has less elements than the url list, those urls without a corresponding
// postData array will return an error.
//
// If the w writers slice contains a single writer, all the responses will be
// written to the single writer. If the writers array contains more than one
// io.Writer, each request will be written to its corresponding writer unless it
// is missing, in that case the request will return an an error. The requests are
// guaranteed to go into the writer in order.
func (a *Agent) PostToWriterGroup(w []io.Writer, urls []string, postData [][]byte) []error {
	//nolint: bodyclose // Next line closes them
	resps, errs := a.PostRequestGroup(urls, postData)
	defer closeHTTPResponseGroup(resps)

	for i, r := range resps {
		if r == nil {
			continue
		}

		var err error
		if len(w) == 1 {
			err = a.readResponse(r, w[0])
		} else {
			if i >= len(w) {
				err = fmt.Errorf("request %d has no writer defined", i)
			} else {
				err = a.readResponse(r, w[i])
			}
		}
		if err != nil {
			errs[i] = fmt.Errorf("writing group response #%d: %w", i, err)
			continue
		}
	}
	return errs
}

// GetGroup behaves just as Get() but takes a group of URLs and performs
// the requests in parallel. The number of simultaneous requests is controlled by
// options.MaxParallel.
func (a *Agent) GetGroup(urls []string) ([][]byte, []error) {
	//nolint: bodyclose // Next line closes them
	resps, errs := a.GetRequestGroup(urls)
	defer closeHTTPResponseGroup(resps)

	c := make([][]byte, len(urls))
	for i, r := range resps {
		if r != nil {
			d, err := a.readResponseToByteArray(r)
			if err != nil {
				errs[i] = fmt.Errorf("reading group response #%d: %w", i, err)
				continue
			}
			c[i] = d
		}
	}
	return c, errs
}

// GetToWriterGroup behaves just as GetToWriter() but takes a group of URLs
// and performs the requests in parallel. The number of simultaneous requests
// is controlled by options.MaxParallel.
//
// If the w writers slice contains a single writer, all the responses will be
// written to the single writer. If the writers array contains more than one
// io.Writer, each request will be written to its corresponding writer unless it
// is missing in which case the request will return an an error. The requests are
// guaranteed to go into the writer in order.
func (a *Agent) GetToWriterGroup(w []io.Writer, urls []string) []error {
	//nolint: bodyclose
	resps, errs := a.GetRequestGroup(urls)
	defer closeHTTPResponseGroup(resps)

	for i, r := range resps {
		if r == nil {
			continue
		}

		var err error
		if len(w) == 1 {
			err = a.readResponse(r, w[0])
		} else {
			if i >= len(w) {
				err = fmt.Errorf("request %d has no writer defined", i)
			} else {
				err = a.readResponse(r, w[i])
			}
		}
		if err != nil {
			errs[i] = fmt.Errorf("writing group response #%d: %w", i, err)
			continue
		}
	}
	return errs
}
