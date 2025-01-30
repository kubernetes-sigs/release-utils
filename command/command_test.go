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

package command

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSuccess(t *testing.T) {
	res, err := New("echo", "hi").Run()
	require.NoError(t, err)
	require.True(t, res.Success())
	require.Zero(t, res.ExitCode())
}

func TestSuccessPipe(t *testing.T) {
	res, err := New("echo", "-n", "hi").
		Pipe("cat").
		Pipe("cat").
		Run()
	require.NoError(t, err)
	require.True(t, res.Success())
	require.Zero(t, res.ExitCode())
	require.Equal(t, "hi", res.Output())
}

func TestFailurePipeWrongCommand(t *testing.T) {
	res, err := New("echo", "-n", "hi").
		Pipe("wrong").
		Run()
	require.Error(t, err)
	require.Nil(t, res)
}

func TestFailurePipeWrongArgument(t *testing.T) {
	res, err := New("echo", "-n", "hi").
		Pipe("cat", "--wrong").
		Run()
	require.NoError(t, err)
	require.False(t, res.Success())
	require.Empty(t, res.Output())
	require.NotEmpty(t, res.Error())
}

func TestSuccessVerbose(t *testing.T) {
	res, err := New("echo", "hi").Verbose().Run()
	require.NoError(t, err)
	require.True(t, res.Success())
	require.Zero(t, res.ExitCode())
}

func TestSuccessWithWorkingDir(t *testing.T) {
	res, err := NewWithWorkDir("/", "ls", "-1").Run()
	require.NoError(t, err)
	require.True(t, res.Success())
	require.Zero(t, res.ExitCode())
}

func TestFailureWithWrongWorkingDir(t *testing.T) {
	res, err := NewWithWorkDir("/should/not/exist", "ls", "-1").Run()
	require.Error(t, err)
	require.Nil(t, res)
}

func TestSuccessSilent(t *testing.T) {
	res, err := New("echo", "hi").RunSilent()
	require.NoError(t, err)
	require.True(t, res.Success())
}

func TestSuccessSeparated(t *testing.T) {
	res, err := New("echo", "hi").RunSilent()
	require.NoError(t, err)
	require.True(t, res.Success())
}

func TestSuccessSingleArgument(t *testing.T) {
	res, err := New("echo").Run()
	require.NoError(t, err)
	require.True(t, res.Success())
}

func TestSuccessNoArgument(t *testing.T) {
	res, err := New("").Run()
	require.Error(t, err)
	require.Nil(t, res)
}

func TestSuccessOutput(t *testing.T) {
	res, err := New("echo", "-n", "hello world").Run()
	require.NoError(t, err)
	require.Equal(t, "hello world", res.Output())
}

func TestSuccessOutputTrimNL(t *testing.T) {
	res, err := New("echo", "-n", "hello world\n").Run()
	require.NoError(t, err)
	require.Equal(t, "hello world", res.OutputTrimNL())
}

func TestSuccessError(t *testing.T) {
	res, err := New("cat", "/not/valid").Run()
	require.NoError(t, err)
	require.Empty(t, res.Output())
	require.Contains(t, res.Error(), "No such file")
}

func TestSuccessOutputSeparated(t *testing.T) {
	res, err := New("echo", "-n", "hello").Run()
	require.NoError(t, err)
	require.Equal(t, "hello", res.Output())
}

func TestFailureStdErr(t *testing.T) {
	res, err := New("cat", "/not/valid").Run()
	require.NoError(t, err)
	require.False(t, res.Success())
	require.Equal(t, 1, res.ExitCode())
}

func TestFailureNotExisting(t *testing.T) {
	res, err := New("/not/valid").Run()
	require.Error(t, err)
	require.Nil(t, res)
}

func TestSuccessExecute(t *testing.T) {
	err := Execute("echo", "-n", "hi", "ho")
	require.NoError(t, err)
}

func TestFailureExecute(t *testing.T) {
	err := Execute("cat", "/not/invalid")
	require.Error(t, err)
}

func TestAvailableSuccessValidCommand(t *testing.T) {
	res := Available("echo")
	require.True(t, res)
}

func TestAvailableSuccessEmptyCommands(t *testing.T) {
	res := Available()
	require.True(t, res)
}

func TestAvailableFailure(t *testing.T) {
	res := Available("echo", "this-command-should-not-exist")
	require.False(t, res)
}

func TestSuccessRunSuccess(t *testing.T) {
	require.NoError(t, New("echo", "hi").RunSuccess())
}

func TestFailureRunSuccess(t *testing.T) {
	require.Error(t, New("cat", "/not/available").RunSuccess())
}

func TestSuccessRunSilentSuccess(t *testing.T) {
	require.NoError(t, New("echo", "hi").RunSilentSuccess())
}

func TestFailureRunSuccessSilent(t *testing.T) {
	require.Error(t, New("cat", "/not/available").RunSilentSuccess())
}

func TestSuccessRunSuccessOutput(t *testing.T) {
	res, err := New("echo", "-n", "hi").RunSuccessOutput()
	require.NoError(t, err)
	require.Equal(t, "hi", res.Output())
}

func TestFailureRunSuccessOutput(t *testing.T) {
	res, err := New("cat", "/not/available").RunSuccessOutput()
	require.Error(t, err)
	require.Nil(t, res)
}

func TestSuccessRunSilentSuccessOutput(t *testing.T) {
	res, err := New("echo", "-n", "hi").RunSilentSuccessOutput()
	require.NoError(t, err)
	require.Equal(t, "hi", res.Output())
}

func TestFailureRunSilentSuccessOutput(t *testing.T) {
	res, err := New("cat", "/not/available").RunSilentSuccessOutput()
	require.Error(t, err)
	require.Nil(t, res)
}

func TestSuccessLogWriter(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "log")
	require.NoError(t, err)

	defer func() { require.NoError(t, os.Remove(f.Name())) }()

	res, err := New("echo", "Hello World").AddWriter(f).RunSuccessOutput()
	require.NoError(t, err)

	content, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	require.Equal(t, res.Output(), string(content))
}

func TestSuccessLogWriterMultiple(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "log")
	require.NoError(t, err)

	defer func() { require.NoError(t, os.Remove(f.Name())) }()

	b := &bytes.Buffer{}

	res, err := New("echo", "Hello World").
		AddWriter(f).
		AddWriter(b).
		RunSuccessOutput()
	require.NoError(t, err)

	content, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	require.Equal(t, res.Output(), string(content))
	require.Equal(t, res.Output(), b.String())
}

func TestSuccessLogWriterSilent(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "log")
	require.NoError(t, err)

	defer func() { require.NoError(t, os.Remove(f.Name())) }()

	err = New("echo", "Hello World").AddWriter(f).RunSilentSuccess()
	require.NoError(t, err)

	content, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	require.Empty(t, content)
}

func TestSuccessLogWriterStdErr(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "log")
	require.NoError(t, err)

	defer func() { require.NoError(t, os.Remove(f.Name())) }()

	res, err := New("bash", "-c", ">&2 echo error").
		AddWriter(f).RunSuccessOutput()
	require.NoError(t, err)

	content, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	require.Equal(t, res.Error(), string(content))
}

func TestSuccessLogWriterStdErrAndStdOut(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "log")
	require.NoError(t, err)

	defer func() { require.NoError(t, os.Remove(f.Name())) }()

	res, err := New("bash", "-c", ">&2 echo stderr; echo stdout").
		AddWriter(f).RunSuccessOutput()
	require.NoError(t, err)

	content, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	require.Contains(t, string(content), res.Output())
	require.Contains(t, string(content), res.Error())
}

func TestSuccessLogWriterStdErrAndStdOutOnlyStdErr(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "log")
	require.NoError(t, err)

	defer func() { require.NoError(t, os.Remove(f.Name())) }()

	res, err := New("bash", "-c", ">&2 echo stderr; echo stdout").
		AddErrorWriter(f).RunSuccessOutput()
	require.NoError(t, err)

	content, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	require.Equal(t, res.Error(), string(content))
}

func TestSuccessLogWriterStdErrAndStdOutOnlyStdOut(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "log")
	require.NoError(t, err)

	defer func() { require.NoError(t, os.Remove(f.Name())) }()

	res, err := New("bash", "-c", ">&2 echo stderr; echo stdout").
		AddOutputWriter(f).RunSuccessOutput()
	require.NoError(t, err)

	content, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	require.Equal(t, res.Output(), string(content))
}

func TestCommandsSuccess(t *testing.T) {
	res, err := New("echo", "1").Verbose().
		Add("echo", "2").Add("echo", "3").Run()
	require.NoError(t, err)
	require.True(t, res.Success())
	require.Zero(t, res.ExitCode())
	require.Contains(t, res.Output(), "1")
	require.Contains(t, res.Output(), "2")
	require.Contains(t, res.Output(), "3")
}

func TestCommandsFailure(t *testing.T) {
	res, err := New("echo", "1").Add("wrong").Add("echo", "3").Run()
	require.Error(t, err)
	require.Nil(t, res)
}

func TestEnv(t *testing.T) {
	t.Setenv("ABC", "test") // preserved
	t.Setenv("FOO", "test") // overwritten

	res, err := New("sh", "-c", "echo $TEST; echo $FOO; echo $ABC").
		Env("TEST=123").
		Env("FOO=bar").
		RunSuccessOutput()
	require.NoError(t, err)
	require.Equal(t, "123\nbar\ntest", res.OutputTrimNL())
}

func TestFilterStdout(t *testing.T) {
	cmd, err := New("echo", "-n", "1 2 2 3").Filter("[25]", "0")
	require.NoError(t, err)

	res, err := cmd.Add("echo", "-n", "4 5 6 2 2").Run()
	require.NoError(t, err)
	require.True(t, res.Success())
	require.Zero(t, res.ExitCode())
	require.Equal(t, "\n1 0 0 3\n4 0 6 0 0", res.Output())
}

func TestFilterStderr(t *testing.T) {
	res, err := New("bash", "-c", ">&2 echo -n my secret").Filter("secret", "***")
	require.NoError(t, err)
	out, err := res.RunSilentSuccessOutput()
	require.NoError(t, err)
	require.Equal(t, "my ***", out.Error())
	require.Empty(t, out.Output())
}
