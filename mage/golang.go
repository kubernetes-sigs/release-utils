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
	defaultGolangCILintVersion = "v1.40.1"
	golangciCmd                = "golangci-lint"
	golangciConfig             = ".golangci.yml"
	golangciURLBase            = "https://raw.githubusercontent.com/golangci/golangci-lint"
)

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

func TestGo(verbose bool, pkgs ...string) error {
	verboseFlag := ""
	if verbose {
		verboseFlag = "-v"
	}

	pkgArgs := []string{}
	if len(pkgs) > 0 {
		for _, p := range pkgs {
			pkgArg := fmt.Sprintf("./%s/...", p)
			pkgArgs = append(pkgArgs, pkgArg)
		}
	} else {
		pkgArgs = []string{"./..."}
	}

	cmdArgs := []string{"test"}
	cmdArgs = append(cmdArgs, verboseFlag)
	cmdArgs = append(cmdArgs, pkgArgs...)

	if err := shx.RunV(
		"go",
		cmdArgs...,
	); err != nil {
		return errors.Wrap(err, "running go test")
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
