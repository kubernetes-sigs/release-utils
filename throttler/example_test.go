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

// This package was forked and adapted from the original at
// pkg:golang/github.com/nozzle/throttler@2ea982251481626167b7f83be1434b5c42540c1a
// full commit history has been preserved.

package throttler

import (
	"fmt"
	"os"
)

type httpPkg struct{}

func (httpPkg) Get(_ string) error { return nil }

var http httpPkg

// This example fetches several URLs concurrently,
// using a WaitGroup to block until all the fetches are complete.
//
//nolint:testableexamples // TODO - Rewrite examples
func ExampleWaitGroup() {
}

// This example fetches several URLs concurrently,
// using a Throttler to block until all the fetches are complete.
// Compare to http://golang.org/pkg/sync/#example_WaitGroup
//
//nolint:testableexamples // TODO - Rewrite examples
func ExampleThrottler() {
	urls := []string{
		"http://www.golang.org/",
		"http://www.google.com/",
		"http://www.somestupidname.com/",
	}
	// Create a new Throttler that will get 2 urls at a time
	t := New(2, len(urls))
	for _, url := range urls {
		// Launch a goroutine to fetch the URL.
		go func(url string) {
			// Fetch the URL.
			err := http.Get(url)
			// Let Throttler know when the goroutine completes
			// so it can dispatch another worker
			t.Done(err)
		}(url)
		// Pauses until a worker is available or all jobs have been completed
		// Returning the total number of goroutines that have errored
		// lets you choose to break out of the loop without starting any more
		errorCount := t.Throttle()
		if errorCount > 0 {
			break
		}
	}
}

// This example fetches several URLs concurrently,
// using a Throttler to block until all the fetches are complete
// and checks the errors returned.
// Compare to http://golang.org/pkg/sync/#example_WaitGroup
//
//nolint:testableexamples // TODO - Rewrite examples
func ExampleThrottler_errors() {
	urls := []string{
		"http://www.golang.org/",
		"http://www.google.com/",
		"http://www.somestupidname.com/",
	}
	// Create a new Throttler that will get 2 urls at a time
	t := New(2, len(urls))
	for _, url := range urls {
		// Launch a goroutine to fetch the URL.
		go func(url string) {
			// Let Throttler know when the goroutine completes
			// so it can dispatch another worker
			defer t.Done(nil)
			// Fetch the URL.
			if err := http.Get(url); err != nil {
				fmt.Fprintf(os.Stderr, "error fetching %q: %v", url, err)
			}
		}(url)
		// Pauses until a worker is available or all jobs have been completed
		t.Throttle()
	}

	if t.Err() != nil {
		// Loop through the errors to see the details
		for i, err := range t.Errs() {
			fmt.Printf("error #%d: %s", i, err)
		}
	}
}
