// Package main defines automation targets using Magefile
package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// init performs some sanity checks before running anything.
func init() {
	mustBeInRoot()
}

// Dev groups commands for local development.
type Dev mg.Namespace

// unformatted error.
var errUnformatted = errors.New("some files were unformatted, make sure `go fmt` is run")

// Checks runs various pre-merge checks.
func (Dev) Checks() error {
	if err := sh.Run("go", "vet", "./..."); err != nil {
		return fmt.Errorf("failed to run go vet: %w", err)
	}

	out, err := sh.Output("go", "fmt", "./...")
	if err != nil {
		return fmt.Errorf("failed to run gofmt: %w", err)
	}

	if out != "" {
		return errUnformatted
	}

	return nil
}

// Lint lints the codebase.
func (Dev) Lint() error {
	if err := sh.Run("golangci-lint", "run"); err != nil {
		return fmt.Errorf("failed to run staticcheck: %w", err)
	}

	return nil
}

// Test perform the whole project's unit tests.
func (Dev) Test() error {
	if err := sh.Run(
		"go", "run", "-mod=readonly", "github.com/onsi/ginkgo/v2/ginkgo",
		"-p", "-randomize-all", "-repeat=1", "--fail-on-pending", "--race", "--trace",
		"--junit-report=test-report.xml", "./...",
	); err != nil {
		return fmt.Errorf("failed to run: %w", err)
	}

	return nil
}

// error when wrong version format is used.
var errVersionFormat = errors.New("version must be in format vX,Y,Z")

// Release tags a new version and pushes it.
func (Dev) Release(version string) error {
	if !regexp.MustCompile(`^v([0-9]+).([0-9]+).([0-9]+)$`).Match([]byte(version)) {
		return errVersionFormat
	}

	if err := sh.Run("git", "tag", version); err != nil {
		return fmt.Errorf("failed to tag version: %w", err)
	}

	if err := sh.Run("git", "push", "origin", version); err != nil {
		return fmt.Errorf("failed to push version tag: %w", err)
	}

	return nil
}

// mustBeInRoot checks that the command is run in the project root.
func mustBeInRoot() {
	if _, err := os.Stat("go.mod"); err != nil {
		panic("must be in root, couldn't stat go.mod file: " + err.Error())
	}
}
