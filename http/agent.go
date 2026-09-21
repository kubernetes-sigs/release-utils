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
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
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
	client  *http.Client
	AgentImplementation
}

// AgentImplementation is the actual implementation of the http calls. Every
// request is built from the context it receives, so cancelling the context
// or reaching its deadline aborts the request in flight.
//
//counterfeiter:generate . AgentImplementation
type AgentImplementation interface {
	SendPostRequest(context.Context, *http.Client, string, []byte, string) (*http.Response, error)
	SendGetRequest(context.Context, *http.Client, string) (*http.Response, error)
	SendHeadRequest(context.Context, *http.Client, string) (*http.Response, error)
}

type defaultAgentImplementation struct{}

// agentOptions has the configurable bits of the agent.
type agentOptions struct {
	FailOnHTTPError bool          // Set to true to fail on HTTP Status > 299
	Retries         uint          // Number of times to retry when errors happen
	Timeout         time.Duration // Timeout when fetching URLs
	WaitTime        time.Duration // Initial wait time for backing off on retry
	MaxWaitTime     time.Duration // Max waiting time when backing off on retry
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
	WaitTime:        2 * time.Second,
	MaxWaitTime:     60 * time.Second,
	PostContentType: defaultPostContentType,
	MaxParallel:     5,
}

// NewAgent return a new agent with default options.
func NewAgent() *Agent {
	opts := *defaultAgentOptions

	return &Agent{
		AgentImplementation: &defaultAgentImplementation{},
		options:             &opts,
		client:              &http.Client{Timeout: opts.Timeout},
	}
}

// SetImplementation sets the agent implementation.
func (a *Agent) SetImplementation(impl AgentImplementation) {
	a.AgentImplementation = impl
}

// WithTimeout sets the agent timeout. The new value is also written to the
// agent's http client, so it must not be called while requests are in flight.
//
// The timeout applies to each request attempt on its own. It works alongside
// the context passed to the Context methods: both limits apply and the
// earliest one wins. Set it to zero to let the context alone govern how long
// an operation may take.
func (a *Agent) WithTimeout(timeout time.Duration) *Agent {
	a.options.Timeout = timeout
	if a.client != nil {
		a.client.Timeout = timeout
	}

	return a
}

// WithRetries sets the number of times we'll attempt to fetch the URL.
func (a *Agent) WithRetries(retries uint) *Agent {
	a.options.Retries = retries

	return a
}

// WithWaitTime sets the initial wait time for request retry.
func (a *Agent) WithWaitTime(t time.Duration) *Agent {
	a.options.WaitTime = t

	return a
}

// WithMaxWaitTime sets the maximum wait time for request retry.
func (a *Agent) WithMaxWaitTime(t time.Duration) *Agent {
	a.options.MaxWaitTime = t

	return a
}

// WithFailOnHTTPError determines if the agent fails on HTTP errors (HTTP status not in 200s).
func (a *Agent) WithFailOnHTTPError(flag bool) *Agent {
	a.options.FailOnHTTPError = flag

	return a
}

// WithMaxParallel controls how many requests we do when fetching groups.
func (a *Agent) WithMaxParallel(workers int) *Agent {
	//nolint:gosec // integer overflow highly unlikely
	a.options.MaxParallel = uint(workers)

	return a
}

// WithClient allows callers to set a custom http client in the agent. The
// agent's timeout is set on it, as it is on the default client; adjust it
// with WithTimeout.
//
// Note: this overwrites the timeout on the provided client. If the same
// client is shared across agents, the last agent to call WithClient (or
// WithTimeout) sets the timeout for all of them.
func (a *Agent) WithClient(c *http.Client) *Agent {
	if c != nil {
		c.Timeout = a.options.Timeout
	}

	a.client = c

	return a
}

// Client returns the net/http client preconfigured with the agent options.
// The client is created with the agent and configured by the With* options,
// so calling this while requests are in flight is safe.
func (a *Agent) Client() *http.Client {
	if a.client == nil {
		a.client = &http.Client{Timeout: a.options.Timeout}
	}

	return a.client
}

// Get returns the body of a GET request. It is GetContext with a background
// context.
func (a *Agent) Get(u string) (content []byte, err error) {
	return a.GetContext(context.Background(), u)
}

// GetContext returns the body of a GET request. Cancelling the context or
// reaching its deadline aborts the request, including any retry in progress.
func (a *Agent) GetContext(ctx context.Context, u string) (content []byte, err error) {
	var b bytes.Buffer

	err = a.retryOperation(ctx, func() error {
		b.Reset()

		resp, sendErr := a.SendGetRequest(ctx, a.Client(), u)
		if retryErr := shouldRetry(resp, sendErr); retryErr != nil {
			if resp != nil {
				resp.Body.Close()
			}

			return retryErr
		}

		if readErr := a.readResponse(resp, &b); readErr != nil {
			if shouldRetryReadError(readErr) {
				return readErr
			}

			return retry.Unrecoverable(readErr)
		}

		return nil
	})

	return b.Bytes(), err
}

// GetRequest sends a GET request to a URL and returns the response. It is
// GetRequestContext with a background context.
func (a *Agent) GetRequest(u string) (response *http.Response, err error) {
	return a.GetRequestContext(context.Background(), u)
}

// GetRequestContext sends a GET request to a URL and returns the response.
// Cancelling the context or reaching its deadline aborts the request,
// including any retry in progress.
func (a *Agent) GetRequestContext(ctx context.Context, u string) (response *http.Response, err error) {
	logrus.Debugf("Sending GET request to %s", u)

	return a.retryRequest(ctx, func() (*http.Response, error) {
		return a.SendGetRequest(ctx, a.Client(), u)
	})
}

// Post returns the body of a POST request. It is PostContext with a
// background context.
func (a *Agent) Post(u string, postData []byte) (content []byte, err error) {
	return a.PostContext(context.Background(), u, postData)
}

// PostContext returns the body of a POST request. Cancelling the context or
// reaching its deadline aborts the request, including any retry in progress.
func (a *Agent) PostContext(ctx context.Context, u string, postData []byte) (content []byte, err error) {
	response, err := a.PostRequestContext(ctx, u, postData)
	if err != nil {
		return nil, fmt.Errorf("getting post request: %w", err)
	}
	defer response.Body.Close()

	return a.readResponseToByteArray(response)
}

// PostRequest sends the postData in a POST request to a URL and returns the
// response. It is PostRequestContext with a background context.
func (a *Agent) PostRequest(u string, postData []byte) (response *http.Response, err error) {
	return a.PostRequestContext(context.Background(), u, postData)
}

// PostRequestContext sends the postData in a POST request to a URL and returns
// the response. Cancelling the context or reaching its deadline aborts the
// request, including any retry in progress.
func (a *Agent) PostRequestContext(ctx context.Context, u string, postData []byte) (response *http.Response, err error) {
	logrus.Debugf("Sending POST request to %s", u)

	return a.retryRequest(ctx, func() (*http.Response, error) {
		return a.SendPostRequest(ctx, a.Client(), u, postData, a.options.PostContentType)
	})
}

// retryRequest retries a request using the agent's retry settings. Once the
// context is done no further attempts are made, including waiting out a
// backoff delay, and the context error is returned.
func (a *Agent) retryRequest(ctx context.Context, do func() (*http.Response, error)) (response *http.Response, err error) {
	if a.options.Retries == 0 {
		return do()
	}

	err = retry.Do(func() error {
		//nolint:bodyclose // The API consumer should close the body
		response, err = do()

		return shouldRetry(response, err)
	},
		retry.Context(ctx),
		retry.Attempts(a.options.Retries),
		retry.Delay(a.options.WaitTime),
		retry.MaxDelay(a.options.MaxWaitTime),
		retry.DelayType(retry.BackOffDelay),
		retry.OnRetry(func(attempt uint, err error) {
			logrus.Errorf("Unable to do request (attempt %d/%d): %v", attempt+1, a.options.Retries, err)
		}),
	)

	return response, err
}

// retryOperation retries a generic operation using the agent's retry settings.
// Once the context is done no further attempts are made, including waiting
// out a backoff delay, and the context error is returned.
func (a *Agent) retryOperation(ctx context.Context, do func() error) error {
	if a.options.Retries == 0 {
		return do()
	}

	return retry.Do(do,
		retry.Context(ctx),
		retry.Attempts(a.options.Retries),
		retry.Delay(a.options.WaitTime),
		retry.MaxDelay(a.options.MaxWaitTime),
		retry.DelayType(retry.BackOffDelay),
		retry.OnRetry(func(attempt uint, err error) {
			logrus.Errorf("Unable to do request (attempt %d/%d): %v", attempt+1, a.options.Retries, err)
		}),
	)
}

func shouldRetry(resp *http.Response, err error) error {
	urlErr := &url.Error{}
	if err != nil && errors.As(err, &urlErr) {
		return err
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("retry %d: %s", resp.StatusCode, resp.Status)
	}

	if resp.StatusCode == 0 || (resp.StatusCode >= 500 &&
		resp.StatusCode != http.StatusNotImplemented) {
		return fmt.Errorf("retry unexpected HTTP status %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

func shouldRetryReadError(err error) bool {
	var netErr net.Error

	return errors.As(err, &netErr) && netErr.Timeout() || errors.Is(err, context.DeadlineExceeded)
}

// Head returns the body of a HEAD request. It is HeadContext with a
// background context.
func (a *Agent) Head(u string) (content []byte, err error) {
	return a.HeadContext(context.Background(), u)
}

// HeadContext returns the body of a HEAD request. Cancelling the context or
// reaching its deadline aborts the request, including any retry in progress.
func (a *Agent) HeadContext(ctx context.Context, u string) (content []byte, err error) {
	response, err := a.HeadRequestContext(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("getting head request: %w", err)
	}
	defer response.Body.Close()

	return a.readResponseToByteArray(response)
}

// HeadRequest sends a HEAD request to a URL and returns the response. It is
// HeadRequestContext with a background context.
func (a *Agent) HeadRequest(u string) (response *http.Response, err error) {
	return a.HeadRequestContext(context.Background(), u)
}

// HeadRequestContext sends a HEAD request to a URL and returns the response,
// retrying with exponential backoff. Once the context is done no further
// attempts are made, including waiting out a backoff delay.
func (a *Agent) HeadRequestContext(ctx context.Context, u string) (response *http.Response, err error) {
	logrus.Debugf("Sending HEAD request to %s", u)

	var try uint

	for {
		response, err = a.SendHeadRequest(ctx, a.Client(), u)
		try++

		if err == nil || try >= a.options.Retries {
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
			a.options.Retries-try, waitTime, err.Error(),
		)

		// Wait out the backoff unless the context ends first.
		timer := time.NewTimer(time.Duration(waitTime) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()

			return response, fmt.Errorf("%w (last error: %w)", ctx.Err(), err)
		case <-timer.C:
		}
	}
}

// SendPostRequest sends the actual HTTP post to the server.
func (impl *defaultAgentImplementation) SendPostRequest(
	ctx context.Context, client *http.Client, u string, postData []byte, contentType string,
) (response *http.Response, err error) {
	if contentType == "" {
		contentType = defaultPostContentType
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(postData))
	if err != nil {
		return nil, fmt.Errorf("building post request to %s: %w", u, err)
	}

	req.Header.Set("Content-Type", contentType)

	response, err = client.Do(req)
	if err != nil {
		return response, fmt.Errorf("posting data to %s: %w", u, err)
	}

	return response, nil
}

// SendGetRequest performs the actual request.
func (impl *defaultAgentImplementation) SendGetRequest(
	ctx context.Context, client *http.Client, u string,
) (response *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("building get request to %s: %w", u, err)
	}

	response, err = client.Do(req)
	if err != nil {
		return response, fmt.Errorf("getting %s: %w", u, err)
	}

	return response, nil
}

// SendHeadRequest performs the actual request.
func (impl *defaultAgentImplementation) SendHeadRequest(
	ctx context.Context, client *http.Client, u string,
) (response *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("building head request to %s: %w", u, err)
	}

	response, err = client.Do(req)
	if err != nil {
		return response, fmt.Errorf("sending head request %s: %w", u, err)
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

// GetToWriter sends a GET request and writes the response to an io.Writer.
// It is GetToWriterContext with a background context.
func (a *Agent) GetToWriter(w io.Writer, u string) error {
	return a.GetToWriterContext(context.Background(), w, u)
}

// GetToWriterContext sends a GET request and writes the response to an
// io.Writer. Cancelling the context or reaching its deadline aborts the request.
func (a *Agent) GetToWriterContext(ctx context.Context, w io.Writer, u string) error {
	resp, err := a.SendGetRequest(ctx, a.Client(), u)
	if err != nil {
		return fmt.Errorf("sending GET request: %w", err)
	}

	return a.readResponse(resp, w)
}

// PostToWriter sends a POST request and writes the response to an io.Writer.
// It is PostToWriterContext with a background context.
func (a *Agent) PostToWriter(w io.Writer, u string, postData []byte) error {
	return a.PostToWriterContext(context.Background(), w, u, postData)
}

// PostToWriterContext sends a POST request and writes the response to an
// io.Writer. Cancelling the context or reaching its deadline aborts the request.
func (a *Agent) PostToWriterContext(ctx context.Context, w io.Writer, u string, postData []byte) error {
	resp, err := a.SendPostRequest(ctx, a.Client(), u, postData, a.options.PostContentType)
	if err != nil {
		return fmt.Errorf("sending POST request: %w", err)
	}

	return a.readResponse(resp, w)
}

// GetRequestGroup behaves like agent.GetRequest() but takes a group of URLs
// and performs the requests in parallel. The number of simultaneous requests is
// controlled by options.MaxParallel. It is GetRequestGroupContext with a
// background context.
func (a *Agent) GetRequestGroup(urls []string) ([]*http.Response, []error) {
	//nolint:bodyclose // The API consumer should close the bodies
	return a.GetRequestGroupContext(context.Background(), urls)
}

// GetRequestGroupContext behaves like agent.GetRequestContext() but takes a
// group of URLs and performs the requests in parallel. The number of
// simultaneous requests is controlled by options.MaxParallel. Cancelling the
// context or reaching its deadline aborts every request in the group.
func (a *Agent) GetRequestGroupContext(ctx context.Context, urls []string) ([]*http.Response, []error) {
	//nolint:gosec // integer overflow highly unlikely
	t := throttler.New(int(a.options.MaxParallel), len(urls))
	ret := make([]*http.Response, len(urls))
	errs := make([]error, len(urls))
	m := sync.Mutex{}

	client := a.Client()

	for i := range urls {
		go func(url string) {
			//nolint: bodyclose // We don't close here as we're returning the response
			resp, err := a.SendGetRequest(ctx, client, url)

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

// PostRequestGroup behaves like agent.PostRequest() but takes a group of URLs
// and performs the requests in parallel. The number of simultaneous requests
// is controlled by options.MaxParallel. It is PostRequestGroupContext with a
// background context.
//
// The list of URLs and postData byte arrays are required to be of equal length.
// If postData has less elements than the URL list, the function will exit early,
// failing all requests.
func (a *Agent) PostRequestGroup(urls []string, postData [][]byte) ([]*http.Response, []error) {
	//nolint:bodyclose // The API consumer should close the bodies
	return a.PostRequestGroupContext(context.Background(), urls, postData)
}

// PostRequestGroupContext behaves like agent.PostRequestContext() but takes a
// group of URLs and performs the requests in parallel. The number of
// simultaneous requests is controlled by options.MaxParallel. Cancelling the
// context or reaching its deadline aborts every request in the group.
//
// The list of URLs and postData byte arrays are required to be of equal length.
// If postData has less elements than the URL list, the function will exit early,
// failing all requests.
func (a *Agent) PostRequestGroupContext(ctx context.Context, urls []string, postData [][]byte) ([]*http.Response, []error) {
	ret := make([]*http.Response, len(urls))
	errs := make([]error, len(urls))
	// URLs and postData arrays must be equal in length. If not exit now.
	if len(postData) != len(urls) {
		err := errors.New("unable to perform requests, same number URLs and POST payloads required")
		for i := range urls {
			errs[i] = err
		}

		return ret, errs
	}

	//nolint:gosec // integer overflow highly unlikely
	t := throttler.New(int(a.options.MaxParallel), len(urls))
	m := sync.Mutex{}
	client := a.Client()

	for i := range urls {
		go func(url string, pdata []byte) {
			//nolint: bodyclose // We don't close here as we're returning the raw response
			resp, err := a.SendPostRequest(
				ctx, client, url, pdata, a.options.PostContentType,
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
// options.MaxParallel. It is PostGroupContext with a background context.
//
// The list of URLs and postData byte arrays are expected to be of equal length.
// If postData has less elements than the url list, those urls without a corresponding
// postData array will return an error.
func (a *Agent) PostGroup(urls []string, postData [][]byte) ([][]byte, []error) {
	return a.PostGroupContext(context.Background(), urls, postData)
}

// PostGroupContext behaves just as PostContext() but takes a group of URLs and
// performs the requests in parallel. The number of simultaneous requests is
// controlled by options.MaxParallel. Cancelling the context or reaching its
// deadline aborts every request in the group.
//
// The list of URLs and postData byte arrays are expected to be of equal length.
// If postData has less elements than the url list, those urls without a corresponding
// postData array will return an error.
func (a *Agent) PostGroupContext(ctx context.Context, urls []string, postData [][]byte) ([][]byte, []error) {
	//nolint: bodyclose // Next line closes them
	resps, errs := a.PostRequestGroupContext(ctx, urls, postData)
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
// is controlled by options.MaxParallel. It is PostToWriterGroupContext with a
// background context.
//
// The list of URLs and postData byte arrays are expected to be of equal length.
// If postData has less elements than the url list, those urls without a corresponding
// postData array will return an error.
//
// If the w writers slice contains a single writer, all the responses will be
// written to the single writer. If the writers array contains more than one
// io.Writer, each request will be written to its corresponding writer unless it
// is missing, in that case the request will return an error. The requests are
// guaranteed to go into the writer in order.
func (a *Agent) PostToWriterGroup(w []io.Writer, urls []string, postData [][]byte) []error {
	return a.PostToWriterGroupContext(context.Background(), w, urls, postData)
}

// PostToWriterGroupContext behaves just as PostToWriterContext() but takes a
// group of URLs and performs the requests in parallel. The number of
// simultaneous requests is controlled by options.MaxParallel. Cancelling the
// context or reaching its deadline aborts every request in the group.
//
// The list of URLs and postData byte arrays are expected to be of equal length.
// If postData has less elements than the url list, those urls without a corresponding
// postData array will return an error.
//
// If the w writers slice contains a single writer, all the responses will be
// written to the single writer. If the writers array contains more than one
// io.Writer, each request will be written to its corresponding writer unless it
// is missing, in that case the request will return an error. The requests are
// guaranteed to go into the writer in order.
func (a *Agent) PostToWriterGroupContext(ctx context.Context, w []io.Writer, urls []string, postData [][]byte) []error {
	//nolint: bodyclose // Next line closes them
	resps, errs := a.PostRequestGroupContext(ctx, urls, postData)
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
// options.MaxParallel. It is GetGroupContext with a background context.
func (a *Agent) GetGroup(urls []string) ([][]byte, []error) {
	return a.GetGroupContext(context.Background(), urls)
}

// GetGroupContext behaves just as GetContext() but takes a group of URLs and
// performs the requests in parallel. The number of simultaneous requests is
// controlled by options.MaxParallel. Cancelling the context or reaching its
// deadline aborts every request in the group.
func (a *Agent) GetGroupContext(ctx context.Context, urls []string) ([][]byte, []error) {
	//nolint: bodyclose // Next line closes them
	resps, errs := a.GetRequestGroupContext(ctx, urls)
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
// is controlled by options.MaxParallel. It is GetToWriterGroupContext with a
// background context.
//
// If the w writers slice contains a single writer, all the responses will be
// written to the single writer. If the writers array contains more than one
// io.Writer, each request will be written to its corresponding writer unless it
// is missing in which case the request will return an error. The requests are
// guaranteed to go into the writer in order.
func (a *Agent) GetToWriterGroup(w []io.Writer, urls []string) []error {
	return a.GetToWriterGroupContext(context.Background(), w, urls)
}

// GetToWriterGroupContext behaves just as GetToWriterContext() but takes a
// group of URLs and performs the requests in parallel. The number of
// simultaneous requests is controlled by options.MaxParallel. Cancelling the
// context or reaching its deadline aborts every request in the group.
//
// If the w writers slice contains a single writer, all the responses will be
// written to the single writer. If the writers array contains more than one
// io.Writer, each request will be written to its corresponding writer unless it
// is missing in which case the request will return an error. The requests are
// guaranteed to go into the writer in order.
func (a *Agent) GetToWriterGroupContext(ctx context.Context, w []io.Writer, urls []string) []error {
	//nolint: bodyclose
	resps, errs := a.GetRequestGroupContext(ctx, urls)
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
