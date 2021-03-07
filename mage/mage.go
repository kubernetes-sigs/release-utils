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
	// golangci-lint
	defaultGolangCILintVersion = "v1.38.0"
	golangciCmd                = "golangci-lint"
	golangciConfig             = ".golangci.yml"
	golangciURLBase            = "https://raw.githubusercontent.com/golangci/golangci-lint"

	// repo-infra (used for boilerplate script)
	defaultRepoInfraVersion = "v0.2.1"
	repoInfraURLBase        = "https://raw.githubusercontent.com/kubernetes/repo-infra"

	// zeitgeist
	defaultZeitgeistVersion = "v0.2.0"
	zeitgeistCmd            = "zeitgeist"
	zeitgeistModule         = "sigs.k8s.io/zeitgeist"
)

// EnsureBoilerplateScript downloads copyright header boilerplate script, if
// not already present in the repository.
func EnsureBoilerplateScript(version, boilerplateScript string, forceInstall bool) error {
	found, err := kpath.Exists(kpath.CheckSymlinkOnly, boilerplateScript)
	if err != nil {
		return errors.Wrapf(
			err,
			"checking if copyright header boilerplate script (%s) exists",
			boilerplateScript,
		)
	}

	if !found || forceInstall {
		if version == "" {
			log.Printf(
				"A verify_boilerplate.py version to install was not specified. Using default version: %s",
				defaultRepoInfraVersion,
			)

			version = defaultRepoInfraVersion
		}

		if !strings.HasPrefix(version, "v") {
			return errors.New(
				fmt.Sprintf(
					"repo-infra version (%s) must begin with a 'v'",
					version,
				),
			)
		}

		if _, err := semver.ParseTolerant(version); err != nil {
			return errors.Wrapf(
				err,
				"%s was not SemVer-compliant. Cannot continue.",
				version,
			)
		}

		installURL, err := url.Parse(repoInfraURLBase)
		if err != nil {
			return errors.Wrap(err, "parsing URL")
		}

		installURL.Path = path.Join(
			installURL.Path,
			version,
			"hack",
			"verify_boilerplate.py",
		)

		installCmd := command.New(
			"curl",
			"-sSfL",
			installURL.String(),
			"-o",
			boilerplateScript,
		)

		err = installCmd.RunSuccess()
		if err != nil {
			return errors.Wrap(err, "installing verify_boilerplate.py")
		}
	}

	if err := os.Chmod(boilerplateScript, 0755); err != nil {
		return errors.Wrap(err, "making script executable")
	}

	return nil
}

// Ensure golangci-lint is installed and on the PATH.
func EnsureGolangCILint(version string, forceInstall bool) error {
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

		if _, err := semver.ParseTolerant(version); err != nil {
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
			installURL.String(),
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

	return nil
}

// Ensure zeitgeist is installed and on the PATH.
func EnsureZeitgeist(version string) error {
	if version == "" {
		log.Printf(
			"A zeitgeist version to install was not specified. Using default version: %s",
			defaultZeitgeistVersion,
		)

		version = defaultZeitgeistVersion
	}

	if _, err := semver.ParseTolerant(version); err != nil {
		return errors.Wrapf(
			err,
			"%s was not SemVer-compliant. Cannot continue.",
			version,
		)
	}

	if err := pkg.EnsurePackage(zeitgeistModule, defaultZeitgeistVersion); err != nil {
		return errors.Wrap(err, "ensuring package")
	}

	return nil
}

// RunGolangCILint runs all golang linters
func RunGolangCILint(version string, forceInstall bool, args ...string) error {
	if _, err := kpath.Exists(kpath.CheckSymlinkOnly, golangciConfig); err != nil {
		return errors.Wrapf(
			err,
			"checking if golangci-lint config file (%s) exists",
			golangciConfig,
		)
	}

	if err := EnsureGolangCILint(version, forceInstall); err != nil {
		return errors.Wrap(err, "ensuring golangci-lint is installed")
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
func VerifyBoilerplate(version, binDir, boilerplateDir string, forceInstall bool) error {
	if _, err := kpath.Exists(kpath.CheckSymlinkOnly, boilerplateDir); err != nil {
		return errors.Wrapf(
			err,
			"checking if copyright header boilerplate directory (%s) exists",
			boilerplateDir,
		)
	}

	boilerplateScript := filepath.Join(binDir, "verify_boilerplate.py")

	if err := EnsureBoilerplateScript(version, boilerplateScript, forceInstall); err != nil {
		return errors.Wrap(err, "ensuring copyright header script is installed")
	}

	if err := shx.RunV(
		boilerplateScript,
		"--boilerplate-dir",
		boilerplateDir,
	); err != nil {
		return errors.Wrap(err, "running copyright header checks")
	}

	return nil
}

// VerifyDeps runs zeitgeist to verify dependency versions
func VerifyDeps(version, basePath, configPath string, localOnly bool) error {
	if err := EnsureZeitgeist(version); err != nil {
		return errors.Wrap(err, "ensuring zeitgeist is installed")
	}

	args := []string{"validate"}
	if localOnly {
		args = append(args, "--local")
	}

	if basePath != "" {
		args = append(args, "--base-path", basePath)
	}

	if configPath != "" {
		args = append(args, "--config", configPath)
	}

	if err := shx.RunV(zeitgeistCmd, args...); err != nil {
		return errors.Wrap(err, "running zeitgeist")
	}

	return nil
}

// VerifyGoMod runs `go mod tidy` and `git diff --exit-code go.*` to ensure
// all module updates have been checked in.
func VerifyGoMod(scriptDir string) error {
	if err := shx.RunV("go", "mod", "tidy"); err != nil {
		return errors.Wrap(err, "running go mod tidy")
	}

	if err := shx.RunV("git", "diff", "--exit-code", "go.*"); err != nil {
		return errors.Wrap(err, "running go mod tidy")
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
