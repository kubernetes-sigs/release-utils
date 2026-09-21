/*
Copyright 2024 The Kubernetes Authors.

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

/*
Package http provides a configurable agent to talk to http servers.

# Function Families

It provides three families of functions for the GET, POST and HEAD methods
that return the raw http.Response, the response contents as a byte slice or to
write the response to a writer.

Each of these functions also provide a _Group_ equivalent that takes a list
of URLs and performs the requests in parallel, and a _Context_ equivalent
that takes a context.Context as its first argument. The easiest way to
understand the functions is this expression:

	METHOD[Request|ToWriter][Group][Context]

So, for example, the functions for the POST method include the following
variations, note that the Group variants take and return the same arguments
but in plural form, ie same type but a slice:

	Post(string url, []byte postData) ([]byte, error)
	PostRequest(string url, []byte postData) (*http.Response, error)
	PostToWriter(io.Writer w, string url, []byte postData) error
	PostGroup([]string urls, [][]byte postData) ([][]byte, []error)
	PostRequestGroup([]string urls, [][]byte postData) ([]*http.Response, []error)
	PostToWriterGroup([]io.Writer w, []string urls, [][]byte postData) []error

Each of them has a Context variant with the same arguments after the context:

	PostContext(context.Context ctx, string url, []byte postData) ([]byte, error)
	PostGroupContext(context.Context ctx, []string urls, [][]byte postData) ([][]byte, []error)

The functions without a context are equivalent to calling the Context variant
with context.Background().

# Cancellation and Timeouts

The Context variants honor the context for the whole operation: cancelling it
or reaching its deadline aborts the request in flight, stops any further retry
and interrupts a retry backoff that is waiting. In a Group request every
request in the group is aborted. The returned error wraps the context error,
so it can be checked with errors.Is(err, context.Canceled) or
errors.Is(err, context.DeadlineExceeded).

The context works alongside the agent's own timeout, which is set with
.WithTimeout() and applies to each request attempt individually. Both limits
apply and the earliest one wins. To let the context alone govern how long an
operation may take, disable the per-request timeout:

	agent := http.NewAgent().WithTimeout(0)
	data, err := agent.GetContext(ctx, url)

# Group Requests

All the _Group_ families perform the requests in parallel. The number of
simultaneous requests can be controlled with the .WithMaxParallel(int) option:

	# Create an HTTP agent that performs two requests at a time:
	agent := http.NewAgent().WithMaxParallel(2)

All group requests take arguments in slices and return data and errors in slices
guaranteed to be of the same length and order as the arguments.

To check the returned error slice for success in a single shot the errors.Join()
function comes in handy:

	responses, errs := agent.GetGroup(urlList)
	if errors.Join(errs) != nil {
	   // Handle errors here
	}

# Single and Multiple Writer Output

The ToWriterGroup variants take a list of writers in their first arguments.
Usually, the data returned by the requests will be written to each corresponding
writer in the slice (eg request #5 to writer #5). There is an exception though,
if the writer slice contains a single writer, the data from all requests will
be written - in order - into the single writer. This allows for simple piping to
a single output sink (ie all output to STDOUT).

# Example

The following example shows a code snippet that fetches ten photographs in parallel
and writes them to disk.
*/
package http
