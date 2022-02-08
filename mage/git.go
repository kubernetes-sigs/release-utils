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

package mage

import (
	"github.com/pkg/errors"

	"sigs.k8s.io/release-utils/command"
)

const (
	gitConfigNameKey    = "user.name"
	gitConfigNameValue  = "releng-ci-user"
	gitConfigEmailKey   = "user.email"
	gitConfigEmailValue = "nobody@k8s.io"
)

func CheckGitConfigExists() (bool, error) {
	userName := command.New(
		"git",
		"config",
		"--global",
		"--get",
		gitConfigNameKey,
	)

	stream, err := userName.RunSuccessOutput()
	if err != nil {
		return false, errors.Wrapf(err, "getting git %s", gitConfigNameKey)
	}
	if stream.OutputTrimNL() == "" {
		return false, nil
	}

	userEmail := command.New(
		"git",
		"config",
		"--global",
		"--get",
		gitConfigEmailKey,
	)

	stream, err = userEmail.RunSuccessOutput()
	if err != nil {
		return false, errors.Wrapf(err, "getting git %s", gitConfigEmailKey)
	}
	if stream.OutputTrimNL() == "" {
		return false, nil
	}

	return true, nil
}

func EnsureGitConfig() error {
	exists, err := CheckGitConfigExists()
	if err != nil {
		return errors.Wrap(err, "ensuring git config")
	}
	if exists {
		return nil
	}

	if err := command.New(
		"git",
		"config",
		"--global",
		gitConfigNameKey,
		gitConfigNameValue,
	).RunSuccess(); err != nil {
		return errors.Wrapf(err, "configuring git %s", gitConfigNameKey)
	}

	if err := command.New(
		"git",
		"config",
		"--global",
		gitConfigEmailKey,
		gitConfigEmailValue,
	).RunSuccess(); err != nil {
		return errors.Wrapf(err, "configuring git %s", gitConfigEmailKey)
	}

	return nil
}
