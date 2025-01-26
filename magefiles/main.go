// Package main provides development automation.
package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/advdv/stdgo/stdlo"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"

	//mage:import dev
	"github.com/advdv/stdgo/stdmage/stdmagedev"
)

func init() {
	stdmagedev.Init()
}

// Dev groups commands for local development.
type Dev mg.Namespace

// Release tags a new version and pushes it.
func (Dev) Release() error {
	version := string(stdlo.Must1(os.ReadFile("version.txt")))

	if !regexp.MustCompile(`^v([0-9]+).([0-9]+).([0-9]+)$`).Match([]byte(version)) {
		return fmt.Errorf("invalid version format: %s", version)
	}

	if err := sh.Run("git", "tag", version); err != nil {
		return fmt.Errorf("failed to tag version: %w", err)
	}

	if err := sh.Run("git", "push", "origin", version); err != nil {
		return fmt.Errorf("failed to push version tag: %w", err)
	}

	return nil
}
