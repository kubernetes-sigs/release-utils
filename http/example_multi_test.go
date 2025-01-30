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
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"

	"sigs.k8s.io/release-utils/http"
)

func Example() {
	// This example fetches 10 photographs from flick in parallel
	agent := http.NewAgent()
	w := []io.Writer{}
	urls := []string{
		"https://live.staticflickr.com/65535/53863838503_3490725fab.jpg",
		"https://live.staticflickr.com/65535/53862224352_a9949bb818.jpg",
		"https://live.staticflickr.com/65535/53863076331_570818d62f_w.jpg",
		"https://live.staticflickr.com/65535/53863751331_aa8cc7c233_w.jpg",
		"https://live.staticflickr.com/65535/53862636262_3ec860a652.jpg",
		"https://live.staticflickr.com/65535/53863034561_079ea0a87b_z.jpg",
		"https://live.staticflickr.com/65535/53862940596_5a991b2271_w.jpg",
		"https://live.staticflickr.com/65535/53863423169_90f8e13b7f_z",
		"https://live.staticflickr.com/65535/53863136849_965bd39df1_n.jpg",
		"https://live.staticflickr.com/65535/53863672556_1050bbf01b_n.jpg",
	}

	for i := range urls {
		f, err := os.Create(fmt.Sprintf("/tmp/photo-%d.jpg", i))
		if err != nil {
			logrus.Fatal("error opening file")
		}

		w = append(w, f)
	}

	defer func() {
		for i := range w {
			w[i].(*os.File).Close()
		}
	}()

	errs := agent.GetToWriterGroup(w, urls)
	if errors.Join(errs...) != nil {
		logrus.Fatalf("%d errors fetching photos: %v", len(errs), errors.Join(errs...))
	}
	// output:
}
