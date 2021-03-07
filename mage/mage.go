/*
Copyright 2021 The Kubernetes Authors.

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
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/carolynvs/magex/pkg"
	"github.com/carolynvs/magex/shx"
	"github.com/pkg/errors"

	kpath "k8s.io/utils/path"
	"sigs.k8s.io/release-utils/command"
)

const (
	defaultGolangCILintVersion = "v1.38.0"
	golangciCmd                = "golangci-lint"
	golangciConfig             = ".golangci.yml"
	golangciURLBase            = "https://raw.githubusercontent.com/golangci/golangci-lint"
)

// RunGolangCILint runs all golang linters
func RunGolangCILint(version string, forceInstall bool, args ...string) error {
	_, err := kpath.Exists(kpath.CheckSymlinkOnly, golangciConfig)
	if err != nil {
		return errors.Wrapf(
			err,
			"checking if golangci-lint config file (%s) exists",
			golangciConfig,
		)
	}

	found, err := pkg.IsCommandAvailable(golangciCmd, version)
	if err != nil {
		return errors.Wrap(
			err,
			fmt.Sprintf("checking if %s is available", golangciCmd),
		)
	}

	if !found || forceInstall {
		if version == "" {
			log.Printf(
				"A golangci-lint version to install was not specified. Using default version: %s",
				defaultGolangCILintVersion,
			)

			version = defaultGolangCILintVersion
		}

		if !strings.HasPrefix(version, "v") {
			return errors.New(
				fmt.Sprintf(
					"golangci-lint version (%s) must begin with a 'v'",
					version,
				),
			)
		}

		_, err := semver.ParseTolerant(version)
		if err != nil {
			return errors.Wrapf(
				err,
				"%s was not SemVer-compliant. Cannot continue.",
				version,
			)
		}

		installURL, err := url.Parse(golangciURLBase)
		if err != nil {
			return errors.Wrap(err, "parsing URL")
		}

		installURL.Path = path.Join(installURL.Path, version, "install.sh")

		err = pkg.EnsureGopathBin()
		if err != nil {
			return errors.Wrap(err, "ensuring $GOPATH/bin")
		}

		gopathBin := pkg.GetGopathBin()

		installCmd := command.New(
			"curl",
			"-sSfL",
			installURL.Path,
		).Pipe(
			"sh",
			"-s",
			"--",
			"-b",
			gopathBin,
			version,
		)

		err = installCmd.RunSuccess()
		if err != nil {
			return errors.Wrap(err, "installing golangci-lint")
		}
	}

	if err := shx.RunV(golangciCmd, "version"); err != nil {
		return errors.Wrap(err, "getting golangci-lint version")
	}

	if err := shx.RunV(golangciCmd, "linters"); err != nil {
		return errors.Wrap(err, "listing golangci-lint linters")
	}

	runArgs := []string{"run"}
	runArgs = append(runArgs, args...)

	if err := shx.RunV(golangciCmd, runArgs...); err != nil {
		return errors.Wrap(err, "running golangci-lint linters")
	}

	return nil
}

// VerifyBoilerplate runs copyright header checks
func VerifyBoilerplate(scriptDir string) error {
	wd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "getting working directory")
	}

	scriptDir = filepath.Join(wd, scriptDir)

	boilerplateScript := filepath.Join(scriptDir, "verify-boilerplate.sh")
	if err := shx.RunV(boilerplateScript); err != nil {
		return errors.Wrap(err, "running copyright header checks")
	}

	return nil
}

// VerifyDeps runs zeitgeist to verify dependency versions
func VerifyDeps(scriptDir string) error {
	wd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "getting working directory")
	}

	scriptDir = filepath.Join(wd, scriptDir)

	dependenciesScript := filepath.Join(scriptDir, "verify-dependencies.sh")
	if err := shx.RunV(dependenciesScript); err != nil {
		return errors.Wrap(err, "running external dependencies check")
	}

	return nil
}

// VerifyGoMod run the go module linter
func VerifyGoMod(scriptDir string) error {
	wd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "getting working directory")
	}

	scriptDir = filepath.Join(wd, scriptDir)

	goModScript := filepath.Join(scriptDir, "verify-go-mod.sh")
	if err := shx.RunV(goModScript); err != nil {
		return errors.Wrap(err, "running go module linter")
	}

	return nil
}

// VerifyBuild builds the project for a chosen set of platforms
func VerifyBuild(scriptDir string) error {
	wd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "getting working directory")
	}

	scriptDir = filepath.Join(wd, scriptDir)

	buildScript := filepath.Join(scriptDir, "verify-build.sh")
	if err := shx.RunV(buildScript); err != nil {
		return errors.Wrap(err, "running go build")
	}

	return nil
}

/*
##@ Dependencies

.SILENT: update-deps update-deps-go update-mocks
.PHONY:  update-deps update-deps-go update-mocks

update-deps: update-deps-go ## Update all dependencies for this repo
	echo -e "${COLOR}Commit/PR the following changes:${NOCOLOR}"
	git status --short

update-deps-go: GO111MODULE=on
update-deps-go: ## Update all golang dependencies for this repo
	go get -u -t ./...
	go mod tidy
	go mod verify
	$(MAKE) test-go-unit
	./scripts/update-all.sh

update-mocks: ## Update all generated mocks
	go generate ./...
	for f in $(shell find . -name fake_*.go); do \
		cp scripts/boilerplate/boilerplate.generatego.txt tmp ;\
		cat $$f >> tmp ;\
		mv tmp $$f ;\
	done
*/
