/*
Copyright 2022 The Kubernetes Authors.

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

package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionText(t *testing.T) {
	sut := GetVersionInfo()
	require.NotEmpty(t, sut.String())
}

func TestVersionJSON(t *testing.T) {
	sut := GetVersionInfo()
	json, err := sut.JSONString()

	require.NoError(t, err)
	require.NotEmpty(t, json)
}
