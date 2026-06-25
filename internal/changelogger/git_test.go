package changelogger

import (
	"reflect"
	"testing"
)

func TestFilterReleaseBranches(t *testing.T) {
	t.Parallel()

	branches := filterReleaseBranches([]string{
		"origin/HEAD",
		"feature/IU123456-W000001-assign-to-changelog",
		"origin/release/1.25.0",
		"release/1.25.0",
		"origin/hotfix/1.24.1",
		"feature/ABC-123",
		"hotfix/1.24.0",
		"origin/release/1.23.0",
	}, 10)

	expected := []string{
		"release/1.25.0",
		"origin/hotfix/1.24.1",
		"hotfix/1.24.0",
		"origin/release/1.23.0",
	}
	if !reflect.DeepEqual(branches, expected) {
		t.Fatalf("branches = %#v, want %#v", branches, expected)
	}
}

func TestFilterReleaseBranchesLimit(t *testing.T) {
	t.Parallel()

	branches := filterReleaseBranches([]string{
		"release/1.25.0",
		"release/1.24.0",
		"release/1.23.0",
	}, 2)

	expected := []string{"release/1.25.0", "release/1.24.0"}
	if !reflect.DeepEqual(branches, expected) {
		t.Fatalf("branches = %#v, want %#v", branches, expected)
	}
}
