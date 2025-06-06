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

package hash

import (
	"crypto/sha1" //nolint: gosec
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

// SHA512ForFile returns the hex-encoded sha512 hash for the provided filename.
func SHA512ForFile(filename string) (string, error) {
	return ForFile(filename, sha512.New())
}

// SHA256ForFile returns the hex-encoded sha256 hash for the provided filename.
func SHA256ForFile(filename string) (string, error) {
	return ForFile(filename, sha256.New())
}

// SHA1ForFile returns the hex-encoded sha1 hash for the provided filename.
// TODO: check if we can remove this function.
func SHA1ForFile(filename string) (string, error) {
	return ForFile(filename, sha1.New()) //nolint: gosec
}

// ForFile returns the hex-encoded hash for the provided filename and hasher.
func ForFile(filename string, hasher hash.Hash) (string, error) {
	if hasher == nil {
		return "", errors.New("provided hasher is nil")
	}

	f, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("open file %s: %w", filename, err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			logrus.Warnf("Unable to close file %q: %v", filename, err)
		}
	}()

	hasher.Reset()

	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("hash file %s: %w", filename, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
