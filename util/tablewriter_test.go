/*
Copyright 2025 The Kubernetes Authors.

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

package util

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/olekukonko/tablewriter"
	"github.com/stretchr/testify/require"
)

// compareGolden compares actual output with golden file.
func compareGolden(t *testing.T, actual, goldenFile string) {
	t.Helper()

	goldenPath := filepath.Join("testdata", goldenFile)
	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Failed to read golden file: %s", goldenPath)

	require.Equal(t, string(expected), actual, "Output doesn't match golden file: %s", goldenFile)
}

func TestNewTableWriter(t *testing.T) {
	t.Parallel()

	t.Run("NoOptions", func(t *testing.T) {
		t.Parallel()

		var output bytes.Buffer

		table := NewTableWriter(&output)

		require.NotNil(t, table)
		require.IsType(t, &tablewriter.Table{}, table)

		table.Header("Name", "Age")
		_ = table.Append([]string{"John", "30"})
		_ = table.Render()

		compareGolden(t, output.String(), "no_options.golden")
	})

	t.Run("WithSingleOption", func(t *testing.T) {
		t.Parallel()

		var output bytes.Buffer

		table := NewTableWriter(&output, tablewriter.WithMaxWidth(80))

		require.NotNil(t, table)
		require.IsType(t, &tablewriter.Table{}, table)

		table.Header("Name", "Age")
		_ = table.Append([]string{"John", "30"})
		_ = table.Render()

		compareGolden(t, output.String(), "with_single_option.golden")
	})

	t.Run("WithMultipleOptions", func(t *testing.T) {
		t.Parallel()

		var output bytes.Buffer

		table := NewTableWriter(&output,
			tablewriter.WithHeader([]string{"Name", "Age"}),
			tablewriter.WithMaxWidth(80),
		)

		require.NotNil(t, table)
		require.IsType(t, &tablewriter.Table{}, table)

		_ = table.Append([]string{"John", "30"})
		_ = table.Render()

		compareGolden(t, output.String(), "with_multiple_options.golden")
	})

	t.Run("WithHeaderOption", func(t *testing.T) {
		t.Parallel()

		var output bytes.Buffer

		table := NewTableWriter(&output, tablewriter.WithHeader([]string{"Name", "Age"}))

		require.NotNil(t, table)
		require.IsType(t, &tablewriter.Table{}, table)

		_ = table.Append([]string{"John", "30"})
		_ = table.Render()

		compareGolden(t, output.String(), "with_header_option.golden")
	})

	t.Run("WithFooterOption", func(t *testing.T) {
		t.Parallel()

		var output bytes.Buffer

		table := NewTableWriter(&output, tablewriter.WithFooter([]string{"Total", "1"}))

		require.NotNil(t, table)
		require.IsType(t, &tablewriter.Table{}, table)

		table.Header("Name", "Age")
		_ = table.Append([]string{"John", "30"})
		_ = table.Render()

		compareGolden(t, output.String(), "with_footer_option.golden")
	})

	t.Run("EmptyTable", func(t *testing.T) {
		t.Parallel()

		var output bytes.Buffer

		table := NewTableWriter(&output)

		require.NotNil(t, table)
		require.IsType(t, &tablewriter.Table{}, table)

		table.Header("Name", "Age")
		_ = table.Render()

		compareGolden(t, output.String(), "empty_table.golden")
	})

	t.Run("MultipleRows", func(t *testing.T) {
		t.Parallel()

		var output bytes.Buffer

		table := NewTableWriter(&output)

		require.NotNil(t, table)
		require.IsType(t, &tablewriter.Table{}, table)

		table.Header("Name", "Age", "City")
		_ = table.Append([]string{"John", "30", "New York"})
		_ = table.Append([]string{"Jane", "25", "Boston"})
		_ = table.Append([]string{"Bob", "35", "Chicago"})
		_ = table.Render()

		compareGolden(t, output.String(), "multiple_rows.golden")
	})
}
