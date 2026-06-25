package changelogger

import (
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(name string, args ...string) (string, error)
}

type OSRunner struct{}

func (OSRunner) Run(name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}

	return string(output), nil
}

type Git struct {
	runner Runner
}

func (git Git) LastTag() (string, error) {
	commit, err := git.runner.Run("git", "rev-list", "--tags", "--max-count=1")
	if err != nil {
		return "", fmt.Errorf("получить последний commit тэга: %w", err)
	}

	commit = strings.TrimSpace(commit)
	if commit == "" {
		return "", fmt.Errorf("отсутствует последний тэг")
	}

	tag, err := git.runner.Run("git", "describe", "--tags", commit)
	if err != nil {
		return "", fmt.Errorf("получить последний тэг: %w", err)
	}

	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", fmt.Errorf("отсутствует последний тэг")
	}

	return tag, nil
}

func (git Git) MasterCommit() (string, error) {
	commit, err := git.runner.Run("git", "rev-parse", "origin/master")
	if err != nil {
		return "", fmt.Errorf("получить commit origin/master: %w", err)
	}

	return strings.TrimSpace(commit), nil
}

func (git Git) CurrentBranch() (string, error) {
	branch, err := git.runner.Run("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("получить текущую ветку: %w", err)
	}

	branch = strings.TrimSpace(branch)
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("текущая ветка не определена")
	}

	return branch, nil
}

func (git Git) ReleaseBranches(limit int) ([]string, error) {
	output, err := git.runner.Run(
		"git",
		"for-each-ref",
		"--sort=-committerdate",
		"--format=%(refname:short)",
		"refs/heads",
		"refs/remotes/origin",
	)
	if err != nil {
		return nil, fmt.Errorf("получить список релизных веток: %w", err)
	}

	return filterReleaseBranches(splitLines(output), limit), nil
}

func (git Git) ChangeLines(lastTag string, sourceBranch string, masterCommit string) ([]string, error) {
	logOutput, err := git.runner.Run(
		"git",
		"log",
		"--pretty=format:%h|%an|%s|%cs",
		"--no-merges",
		lastTag+".."+sourceBranch,
	)
	if err != nil {
		return nil, fmt.Errorf("получить список коммитов: %w", err)
	}

	lines := splitLines(logOutput)

	cherryOutput, err := git.runner.Run("git", "cherry", "-v", masterCommit, lastTag)
	if err == nil {
		lines = append(lines, splitLines(cherryOutput)...)
	}

	return lines, nil
}

func (git Git) Checkout(branchName string) error {
	if _, err := git.runner.Run("git", "checkout", branchName); err != nil {
		return fmt.Errorf("переключиться на %s: %w", branchName, err)
	}

	return nil
}

func (git Git) CheckoutRemoteBranch(remoteBranch string) (string, error) {
	localBranch, ok := strings.CutPrefix(remoteBranch, "origin/")
	if !ok {
		return remoteBranch, nil
	}

	output, err := git.runner.Run("git", "branch", "--list", localBranch)
	if err != nil {
		return "", fmt.Errorf("проверить существование ветки %s: %w", localBranch, err)
	}

	if strings.TrimSpace(output) != "" {
		if err := git.Checkout(localBranch); err != nil {
			return "", err
		}

		return localBranch, nil
	}

	if _, err := git.runner.Run("git", "checkout", "--track", "-b", localBranch, remoteBranch); err != nil {
		return "", fmt.Errorf("создать локальную ветку %s от %s: %w", localBranch, remoteBranch, err)
	}

	return localBranch, nil
}

func (git Git) CreateBranch(branchName string, sourceBranch string) error {
	output, err := git.runner.Run("git", "branch", "--list", branchName)
	if err != nil {
		return fmt.Errorf("проверить существование ветки: %w", err)
	}

	if strings.TrimSpace(output) != "" {
		_, err := git.runner.Run("git", "checkout", branchName)
		if err != nil {
			return fmt.Errorf("переключиться на существующую ветку: %w", err)
		}

		return nil
	}

	if _, err := git.runner.Run("git", "checkout", sourceBranch); err != nil {
		return fmt.Errorf("переключиться на %s: %w", sourceBranch, err)
	}

	if _, err := git.runner.Run("git", "checkout", "-b", branchName); err != nil {
		return fmt.Errorf("создать ветку %s: %w", branchName, err)
	}

	return nil
}

func (git Git) Commit(changelogPath string) error {
	if _, err := git.runner.Run("git", "add", changelogPath); err != nil {
		return fmt.Errorf("добавить CHANGELOG.md в индекс: %w", err)
	}

	if _, err := git.runner.Run("git", "commit", "-m", "wip: Отредактирован CHANGELOG.md"); err != nil {
		return fmt.Errorf("создать commit: %w", err)
	}

	return nil
}

func (git Git) Push(branchName string) error {
	if _, err := git.runner.Run("git", "push", "origin", branchName); err != nil {
		return fmt.Errorf("push ветки %s: %w", branchName, err)
	}

	return nil
}

func (git Git) DeleteBranch(branchName string, sourceBranch string) error {
	if _, err := git.runner.Run("git", "checkout", sourceBranch); err != nil {
		return fmt.Errorf("переключиться на %s: %w", sourceBranch, err)
	}

	if _, err := git.runner.Run("git", "branch", "-D", branchName); err != nil {
		return fmt.Errorf("удалить локальную ветку %s: %w", branchName, err)
	}

	return nil
}

func filterReleaseBranches(refs []string, limit int) []string {
	localBranches := make(map[string]struct{})
	for _, ref := range refs {
		if strings.HasPrefix(ref, "release/") || strings.HasPrefix(ref, "hotfix/") {
			localBranches[ref] = struct{}{}
		}
	}

	seen := make(map[string]struct{})
	branches := make([]string, 0, limit)
	for _, ref := range refs {
		if ref == "origin/HEAD" || isAssignToChangelogBranch(ref) || !isReleaseLikeBranch(ref) {
			continue
		}

		if local, ok := strings.CutPrefix(ref, "origin/"); ok {
			if _, exists := localBranches[local]; exists {
				continue
			}
		}

		if _, exists := seen[ref]; exists {
			continue
		}
		seen[ref] = struct{}{}

		branches = append(branches, ref)
		if limit > 0 && len(branches) >= limit {
			break
		}
	}

	return branches
}

func isReleaseLikeBranch(branch string) bool {
	return strings.HasPrefix(branch, "release/") ||
		strings.HasPrefix(branch, "hotfix/") ||
		strings.HasPrefix(branch, "origin/release/") ||
		strings.HasPrefix(branch, "origin/hotfix/")
}

func isAssignToChangelogBranch(branch string) bool {
	return strings.HasPrefix(branch, "feature/") && strings.HasSuffix(branch, "-assign-to-changelog")
}

func splitLines(output string) []string {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}

	return strings.Split(output, "\n")
}
