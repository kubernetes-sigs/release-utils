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
	"os"
	"path/filepath"

	"github.com/carolynvs/magex/shx"
	"github.com/pkg/errors"
)

// Verify runs repository verification scripts
func Verify(scriptDir string) error {
	wd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "getting working directory")
	}

	scriptDir = filepath.Join(wd, scriptDir)

	// Run copyright header checks
	boilerplateScript := filepath.Join(scriptDir, "verify-boilerplate.sh")
	if err := shx.RunV(boilerplateScript); err != nil {
		return errors.Wrap(err, "running copyright header checks")
	}

	// Run zeitgeist to verify dependency versions
	dependenciesScript := filepath.Join(scriptDir, "verify-dependencies.sh")
	if err := shx.RunV(dependenciesScript); err != nil {
		return errors.Wrap(err, "running external dependencies check")
	}

	// Run the go module linter
	goModScript := filepath.Join(scriptDir, "verify-go-mod.sh")
	if err := shx.RunV(goModScript); err != nil {
		return errors.Wrap(err, "running go module linter")
	}

	// Run all golang linters
	golangciScript := filepath.Join(scriptDir, "verify-golangci-lint.sh")
	if err := shx.RunV(golangciScript); err != nil {
		return errors.Wrap(err, "running golangci-lint linter")
	}

	// Build the project for a chosen set of platforms
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
