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

package http_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"net/http/httptest"

	"sigs.k8s.io/release-utils/http"
)

func Example() {
	// This example fetches ten photographs in parallel, each into its own
	// writer. The photographs come from a local test server so the example
	// runs anywhere; point the URLs at any server to fetch real content.
	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		fmt.Fprintf(w, "photo %s", r.URL.Path) //nolint:gosec // test server echoing the path back
	}))
	defer server.Close()

	urls := make([]string, 10)
	for i := range urls {
		urls[i] = fmt.Sprintf("%s/photo-%d.jpg", server.URL, i)
	}

	// Fetch two photographs at a time.
	agent := http.NewAgent().WithMaxParallel(2)

	// One writer per URL, filled in the same order as the URLs.
	buffers := make([]bytes.Buffer, len(urls))
	writers := make([]io.Writer, 0, len(urls))

	for i := range buffers {
		writers = append(writers, &buffers[i])
	}

	errs := agent.GetToWriterGroup(writers, urls)
	if err := errors.Join(errs...); err != nil {
		fmt.Println("fetching photos:", err)

		return
	}

	for i := range buffers {
		fmt.Println(buffers[i].String())
	}

	// Output:
	// photo /photo-0.jpg
	// photo /photo-1.jpg
	// photo /photo-2.jpg
	// photo /photo-3.jpg
	// photo /photo-4.jpg
	// photo /photo-5.jpg
	// photo /photo-6.jpg
	// photo /photo-7.jpg
	// photo /photo-8.jpg
	// photo /photo-9.jpg
}
