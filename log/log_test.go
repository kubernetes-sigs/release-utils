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

package log_test

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/release-utils/log"
)

func TestToFile(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "log-test-")
	require.NoError(t, err)

	defer os.Remove(file.Name())

	require.NoError(t, log.SetupGlobalLogger("info"))
	require.NoError(t, log.ToFile(file.Name()))
	logrus.Info("test")

	content, err := os.ReadFile(file.Name())
	require.NoError(t, err)

	require.Contains(t, string(content), "info")
	require.Contains(t, string(content), "test")
}
