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

package http

import (
	"bytes"
)

// GetURLResponse performs a get request and returns the response contents as a
// string if successful.
//
// Deprecated: Use http.Agent.Get() instead. This function will be removed in a
// future version of this package.
func GetURLResponse(url string, trim bool) (string, error) {
	resp, err := NewAgent().Get(url)
	if err != nil {
		return "", err
	}

	if trim {
		resp = bytes.TrimSpace(resp)
	}

	return string(resp), nil
}
