/*
Copyright 2019 The Kubernetes Authors.

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

package env

import (
	"testing"

	"github.com/stretchr/testify/require"

	"sigs.k8s.io/release-utils/env/internal"
	"sigs.k8s.io/release-utils/env/internal/internalfakes"
)

func TestDefault(t *testing.T) {
	for _, tc := range []struct {
		prepare      func(*internalfakes.FakeImpl)
		defaultValue string
		expected     string
	}{
		{ // default LookupEnvReturns empty string and false
			prepare: func(mock *internalfakes.FakeImpl) {
				mock.LookupEnvReturns("", false)
			},
			defaultValue: "default",
			expected:     "default",
		},
		{ // default LookupEnvReturns empty string and true
			prepare: func(mock *internalfakes.FakeImpl) {
				mock.LookupEnvReturns("", true)
			},
			defaultValue: "default",
			expected:     "default",
		},
		{ // default LookupEnvReturns string and false
			prepare: func(mock *internalfakes.FakeImpl) {
				mock.LookupEnvReturns("value", false)
			},
			defaultValue: "default",
			expected:     "default",
		},
		{ // value is set
			prepare: func(mock *internalfakes.FakeImpl) {
				mock.LookupEnvReturns("value", true)
			},
			defaultValue: "default",
			expected:     "value",
		},
	} {
		mock := &internalfakes.FakeImpl{}
		tc.prepare(mock)
		internal.Impl = mock

		res := Default("key", tc.defaultValue)
		require.Equal(t, tc.expected, res)
	}
}

func TestIsSet(t *testing.T) {
	for _, tc := range []struct {
		prepare  func(*internalfakes.FakeImpl)
		expected bool
	}{
		{ // LookupEnvReturns false
			prepare: func(mock *internalfakes.FakeImpl) {
				mock.LookupEnvReturns("", false)
			},
			expected: false,
		},
		{ // LookupEnvReturns true
			prepare: func(mock *internalfakes.FakeImpl) {
				mock.LookupEnvReturns("", true)
			},
			expected: true,
		},
	} {
		mock := &internalfakes.FakeImpl{}
		tc.prepare(mock)
		internal.Impl = mock

		res := IsSet("key")
		require.Equal(t, tc.expected, res)
	}
}
