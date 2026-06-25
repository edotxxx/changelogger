package changelogger

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAskSourceBranchDefaultsToDevelop(t *testing.T) {
	t.Parallel()

	runner := newRecordingRunner(map[string]runnerResponse{
		commandKey("git", "rev-parse", "--abbrev-ref", "HEAD"): {
			output: "feature/ABC-123-something\n",
		},
		commandKey("git", "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)", "refs/heads", "refs/remotes/origin"): {
			output: "release/1.25.0\nrelease/1.24.0\n",
		},
	})
	var output strings.Builder
	app := NewApp(nil, strings.NewReader("\n"), &output, time.Now, runner)

	branch, err := app.askSourceBranch()
	if err != nil {
		t.Fatal(err)
	}

	if branch != "develop" {
		t.Fatalf("branch = %q, want develop", branch)
	}

	assertContains(t, output.String(), "1 - develop [по умолчанию]")
	assertContains(t, output.String(), "2 - release/1.25.0")
	assertContains(t, output.String(), "4 - текущая ветка: feature/ABC-123-something")
	assertContains(t, output.String(), "Выбор [1]:")
}

func TestAskSourceBranchDoesNotOfferCurrentAssignToChangelogBranch(t *testing.T) {
	t.Parallel()

	runner := newRecordingRunner(map[string]runnerResponse{
		commandKey("git", "rev-parse", "--abbrev-ref", "HEAD"): {
			output: "feature/IU123456-W000001-assign-to-changelog\n",
		},
		commandKey("git", "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)", "refs/heads", "refs/remotes/origin"): {
			output: "release/1.25.0\n",
		},
	})
	var output strings.Builder
	app := NewApp(nil, strings.NewReader("\n"), &output, time.Now, runner)

	branch, err := app.askSourceBranch()
	if err != nil {
		t.Fatal(err)
	}

	if branch != "develop" {
		t.Fatalf("branch = %q, want develop", branch)
	}

	text := output.String()
	assertContains(t, text, "служебная, она не будет предложена")
	if strings.Contains(text, "текущая ветка:") {
		t.Fatalf("current assign-to-changelog branch should not be offered: %s", text)
	}
}

func TestAskSourceBranchCreatesLocalBranchForRemoteChoice(t *testing.T) {
	t.Parallel()

	runner := newRecordingRunner(map[string]runnerResponse{
		commandKey("git", "rev-parse", "--abbrev-ref", "HEAD"): {
			output: "HEAD\n",
			err:    errors.New("текущая ветка не определена"),
		},
		commandKey("git", "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)", "refs/heads", "refs/remotes/origin"): {
			output: "origin/release/1.25.0\n",
		},
		commandKey("git", "branch", "--list", "release/1.25.0"):                                   {},
		commandKey("git", "checkout", "--track", "-b", "release/1.25.0", "origin/release/1.25.0"): {},
	})
	var output strings.Builder
	app := NewApp(nil, strings.NewReader("2\n"), &output, time.Now, runner)

	branch, err := app.askSourceBranch()
	if err != nil {
		t.Fatal(err)
	}

	if branch != "release/1.25.0" {
		t.Fatalf("branch = %q, want release/1.25.0", branch)
	}

	assertContains(t, output.String(), "2 - origin/release/1.25.0")
}

func TestAskManualSourceBranchRejectsAssignToChangelogBranch(t *testing.T) {
	t.Parallel()

	app := NewApp(nil, strings.NewReader("feature/IU123456-W000001-assign-to-changelog\n"), &strings.Builder{}, time.Now, newRecordingRunner(nil))

	_, err := app.askManualSourceBranch()
	if err == nil {
		t.Fatal("expected error")
	}

	assertContains(t, err.Error(), "не может быть источником changelog")
}

func TestRunReturnsToLocalSourceBranchAfterRemoteSourceCancellation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.env", []byte("CHANGELOG_PATH=./CHANGELOG.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/CHANGELOG.md", []byte("# История изменений\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	runner := newRecordingRunner(map[string]runnerResponse{
		commandKey("git", "rev-parse", "--abbrev-ref", "HEAD"): {
			output: "HEAD\n",
			err:    errors.New("текущая ветка не определена"),
		},
		commandKey("git", "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)", "refs/heads", "refs/remotes/origin"): {
			output: "origin/release/1.25.0\n",
		},
		commandKey("git", "branch", "--list", "release/1.25.0"):                                   {},
		commandKey("git", "checkout", "--track", "-b", "release/1.25.0", "origin/release/1.25.0"): {},
		commandKey("git", "rev-list", "--tags", "--max-count=1"): {
			output: "tagcommit\n",
		},
		commandKey("git", "describe", "--tags", "tagcommit"): {
			output: "1.2.3\n",
		},
		commandKey("git", "rev-parse", "origin/master"): {
			output: "mastercommit\n",
		},
		commandKey("git", "branch", "--list", "feature/IU123456-W000001-assign-to-changelog"): {},
		commandKey("git", "checkout", "release/1.25.0"):                                       {},
		commandKey("git", "checkout", "-b", "feature/IU123456-W000001-assign-to-changelog"):   {},
		commandKey("git", "log", "--pretty=format:%h|%an|%s|%cs", "--no-merges", "1.2.3..release/1.25.0"): {
			output: "abc|Me|[PROJ-IU123456-W000001] fix: исправлен расчет|2026-06-25\n",
		},
		commandKey("git", "cherry", "-v", "mastercommit", "1.2.3"): {},
	})
	input := strings.NewReader("2\nIU123456-W000001\n1\nn\n")
	app := NewApp(nil, input, &strings.Builder{}, func() time.Time {
		return time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	}, runner)

	err := app.Run()
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	assertContains(t, err.Error(), "выполнение команды отменено")

	if runner.called(commandKey("git", "checkout", "origin/release/1.25.0")) {
		t.Fatalf("remote branch should not be checked out directly, calls: %v", runner.calls)
	}
	if !runner.calledAfter(commandKey("git", "checkout", "-b", "feature/IU123456-W000001-assign-to-changelog"), commandKey("git", "checkout", "release/1.25.0")) {
		t.Fatalf("expected checkout release/1.25.0 after changelog branch creation, calls: %v", runner.calls)
	}
}

func TestRunReturnsToSourceBranchAfterCancellation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.env", []byte("CHANGELOG_PATH=./CHANGELOG.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/CHANGELOG.md", []byte("# История изменений\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	runner := newRecordingRunner(map[string]runnerResponse{
		commandKey("git", "rev-list", "--tags", "--max-count=1"): {
			output: "tagcommit\n",
		},
		commandKey("git", "describe", "--tags", "tagcommit"): {
			output: "1.2.3\n",
		},
		commandKey("git", "rev-parse", "origin/master"): {
			output: "mastercommit\n",
		},
		commandKey("git", "rev-parse", "--abbrev-ref", "HEAD"): {
			output: "feature/IU123456-W000001-assign-to-changelog\n",
		},
		commandKey("git", "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)", "refs/heads", "refs/remotes/origin"): {
			output: "release/1.25.0\n",
		},
		commandKey("git", "branch", "--list", "feature/IU123456-W000001-assign-to-changelog"): {},
		commandKey("git", "checkout", "develop"):                                              {},
		commandKey("git", "checkout", "-b", "feature/IU123456-W000001-assign-to-changelog"):   {},
		commandKey("git", "log", "--pretty=format:%h|%an|%s|%cs", "--no-merges", "1.2.3..develop"): {
			output: "abc|Me|[PROJ-IU123456-W000001] fix: исправлен расчет|2026-06-25\n",
		},
		commandKey("git", "cherry", "-v", "mastercommit", "1.2.3"): {},
	})
	input := strings.NewReader("\nIU123456-W000001\n1\nn\n")
	app := NewApp(nil, input, &strings.Builder{}, func() time.Time {
		return time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	}, runner)

	err := app.Run()
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	assertContains(t, err.Error(), "выполнение команды отменено")

	if !runner.calledAfter(commandKey("git", "checkout", "-b", "feature/IU123456-W000001-assign-to-changelog"), commandKey("git", "checkout", "develop")) {
		t.Fatalf("expected checkout develop after changelog branch creation, calls: %v", runner.calls)
	}
}

func TestRunKeepsChangelogBranchWhenCommitDeclinedAfterChangelogConfirmation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.env", []byte("CHANGELOG_PATH=./CHANGELOG.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/CHANGELOG.md", []byte("# История изменений\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	runner := newRecordingRunner(map[string]runnerResponse{
		commandKey("git", "rev-parse", "--abbrev-ref", "HEAD"): {
			output: "feature/IU123456-W000001-assign-to-changelog\n",
		},
		commandKey("git", "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)", "refs/heads", "refs/remotes/origin"): {
			output: "release/1.25.0\n",
		},
		commandKey("git", "rev-list", "--tags", "--max-count=1"): {
			output: "tagcommit\n",
		},
		commandKey("git", "describe", "--tags", "tagcommit"): {
			output: "1.2.3\n",
		},
		commandKey("git", "rev-parse", "origin/master"): {
			output: "mastercommit\n",
		},
		commandKey("git", "branch", "--list", "feature/IU123456-W000001-assign-to-changelog"): {},
		commandKey("git", "checkout", "develop"):                                              {},
		commandKey("git", "checkout", "-b", "feature/IU123456-W000001-assign-to-changelog"):   {},
		commandKey("git", "log", "--pretty=format:%h|%an|%s|%cs", "--no-merges", "1.2.3..develop"): {
			output: "abc|Me|[PROJ-IU123456-W000001] fix: исправлен расчет|2026-06-25\n",
		},
		commandKey("git", "cherry", "-v", "mastercommit", "1.2.3"): {},
	})
	input := strings.NewReader("\nIU123456-W000001\n1\ny\nn\n")
	var output strings.Builder
	app := NewApp(nil, input, &output, func() time.Time {
		return time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	}, runner)

	if err := app.Run(); err != nil {
		t.Fatal(err)
	}

	if runner.called(commandKey("git", "add", "./CHANGELOG.md")) {
		t.Fatalf("commit should not be started when commit is declined, calls: %v", runner.calls)
	}
	if runner.calledAfter(commandKey("git", "checkout", "-b", "feature/IU123456-W000001-assign-to-changelog"), commandKey("git", "checkout", "develop")) {
		t.Fatalf("source branch should not be restored after changelog confirmation, calls: %v", runner.calls)
	}
	assertContains(t, output.String(), "Коммит не создан")
}

func TestRunKeepsChangelogBranchWhenPushDeclined(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.env", []byte("CHANGELOG_PATH=./CHANGELOG.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/CHANGELOG.md", []byte("# История изменений\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	runner := newRecordingRunner(map[string]runnerResponse{
		commandKey("git", "rev-parse", "--abbrev-ref", "HEAD"): {
			output: "feature/IU123456-W000001-assign-to-changelog\n",
		},
		commandKey("git", "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)", "refs/heads", "refs/remotes/origin"): {
			output: "release/1.25.0\n",
		},
		commandKey("git", "rev-list", "--tags", "--max-count=1"): {
			output: "tagcommit\n",
		},
		commandKey("git", "describe", "--tags", "tagcommit"): {
			output: "1.2.3\n",
		},
		commandKey("git", "rev-parse", "origin/master"): {
			output: "mastercommit\n",
		},
		commandKey("git", "branch", "--list", "feature/IU123456-W000001-assign-to-changelog"): {},
		commandKey("git", "checkout", "develop"):                                              {},
		commandKey("git", "checkout", "-b", "feature/IU123456-W000001-assign-to-changelog"):   {},
		commandKey("git", "log", "--pretty=format:%h|%an|%s|%cs", "--no-merges", "1.2.3..develop"): {
			output: "abc|Me|[PROJ-IU123456-W000001] fix: исправлен расчет|2026-06-25\n",
		},
		commandKey("git", "cherry", "-v", "mastercommit", "1.2.3"):            {},
		commandKey("git", "add", "./CHANGELOG.md"):                            {},
		commandKey("git", "commit", "-m", "wip: Отредактирован CHANGELOG.md"): {},
	})
	input := strings.NewReader("\nIU123456-W000001\n1\ny\ny\nn\n")
	var output strings.Builder
	app := NewApp(nil, input, &output, func() time.Time {
		return time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	}, runner)

	if err := app.Run(); err != nil {
		t.Fatal(err)
	}

	if runner.called(commandKey("git", "push", "origin", "feature/IU123456-W000001-assign-to-changelog")) {
		t.Fatalf("branch should not be pushed when push is declined, calls: %v", runner.calls)
	}
	if runner.called(commandKey("git", "branch", "-D", "feature/IU123456-W000001-assign-to-changelog")) {
		t.Fatalf("changelog branch should not be deleted when push is declined, calls: %v", runner.calls)
	}
	if runner.calledAfter(commandKey("git", "checkout", "-b", "feature/IU123456-W000001-assign-to-changelog"), commandKey("git", "checkout", "develop")) {
		t.Fatalf("source branch should not be restored after changelog confirmation, calls: %v", runner.calls)
	}
	assertContains(t, output.String(), "Push не выполнен")
}

type runnerResponse struct {
	output string
	err    error
}

type recordingRunner struct {
	responses map[string]runnerResponse
	calls     []string
}

func newRecordingRunner(responses map[string]runnerResponse) *recordingRunner {
	if responses == nil {
		responses = make(map[string]runnerResponse)
	}

	return &recordingRunner{responses: responses}
}

func (runner *recordingRunner) Run(name string, args ...string) (string, error) {
	key := commandKey(name, args...)
	runner.calls = append(runner.calls, key)

	response, ok := runner.responses[key]
	if !ok {
		return "", errors.New("unexpected command: " + key)
	}

	return response.output, response.err
}

func (runner *recordingRunner) calledAfter(first string, second string) bool {
	firstIndex := -1
	for index, call := range runner.calls {
		if call == first && firstIndex < 0 {
			firstIndex = index
			continue
		}

		if firstIndex >= 0 && call == second {
			return true
		}
	}

	return false
}

func (runner *recordingRunner) called(command string) bool {
	for _, call := range runner.calls {
		if call == command {
			return true
		}
	}

	return false
}

func commandKey(name string, args ...string) string {
	return name + " " + strings.Join(args, "\x00")
}
