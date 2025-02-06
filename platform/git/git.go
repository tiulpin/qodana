/*
 * Copyright 2021-2024 JetBrains s.r.o.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package git

import (
	"github.com/JetBrains/qodana-cli/v2024/platform/utils"
	log "github.com/sirupsen/logrus"
	"strings"
)

// gitRun runs the git command in the given directory and returns an error if any.
func gitRun(cwd string, command []string, logdir string) (string, string, error) {
	args := []string{"git"}
	args = append(args, command...)
	logger, err := LOGGER.GetLogger(logdir, "git")
	if err != nil {
		log.Errorf("Failed to create git logger: %v", err)
		return "", "", err
	}
	stdout, stderr, _, err := utils.RunCmdRedirectOutput(cwd, args...)
	if logger != nil {
		logger.Printf("Executing command: %v", args)
		logger.Println(stdout)
	}
	if stderr != "" {
		if logger != nil {
			logger.Error(stderr + "\n")
		} else {
			log.Error(stderr)
		}
	}
	if err != nil {
		log.Errorf("Error executing git command %s: %s", strings.Join(args, " "), err)
		return stdout, stderr, err
	}
	return stdout, stderr, nil
}

// Reset resets the git repository to the given commit.
func Reset(cwd string, sha string, logdir string) error {
	_, _, err := gitRun(cwd, []string{"reset", "--soft", sha}, logdir)
	return err
}

// ResetBack aborts the git reset.
func ResetBack(cwd string, logdir string) error {
	_, _, err := gitRun(cwd, []string{"reset", "'HEAD@{1}'"}, logdir)
	return err
}

// Checkout checks out the given commit / branch.
func Checkout(cwd string, where string, force bool, logdir string) error {
	var err error
	if !force {
		_, _, err = gitRun(cwd, []string{"checkout", where}, logdir)
	} else {
		_, _, err = gitRun(cwd, []string{"checkout", "-f", where}, logdir)
	}
	return err
}

// Clean cleans the git repository.
func Clean(cwd string, logdir string) error {
	_, _, err := gitRun(cwd, []string{"clean", "-fdx"}, logdir)
	return err
}

// Revisions returns the list of commits of the git repository in chronological order.
func Revisions(cwd string) []string {
	return utils.Reverse(Log(cwd, "%H", 0))
}

// Root returns absolute path of repo root
func Root(cwd string, logdir string) (string, error) {
	stdout, _, err := gitRun(cwd, []string{"rev-parse", "--show-toplevel"}, logdir)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

// RemoteUrl returns the remote url of the git repository.
func RemoteUrl(cwd string, logdir string) (string, error) {
	stdout, _, err := gitRun(cwd, []string{"remote", "get-url", "origin"}, logdir)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

// Branch returns the current branch of the git repository.
func Branch(cwd string, logdir string) (string, error) {
	stdout, _, err := gitRun(cwd, []string{"rev-parse", "--abbrev-ref", "HEAD"}, logdir)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

func CurrentRevision(cwd string, logdir string) (string, error) {
	stdout, _, err := gitRun(cwd, []string{"rev-parse", "HEAD"}, logdir)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

// RevisionExists returns true when revision exists in history.
func RevisionExists(cwd string, revision string, logdir string) bool {
	_, stderr, err := gitRun(cwd, []string{"show", "--no-patch", revision}, logdir)
	if strings.Contains(stderr, revision) || strings.Contains(stderr, "fatal:") || err != nil {
		return false
	}
	return true
}
