package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Invoked by hub's TestSkillsCLIThroughFabio with a disposable project and PAT.
// Build this test binary with useV0SkillApis=false (a Go source overlay is enough).
func TestSkillsLiveAPI(t *testing.T) {
	endpoint, project := os.Getenv("NOTECLI_SKILLS_TEST_URL"), os.Getenv("NOTECLI_SKILLS_TEST_PROJECT")
	if endpoint == "" || project == "" {
		t.Skip("requires a disposable hub test project")
	}
	if useV0SkillApis {
		t.Fatal("live Skills contract test requires the V1 build")
	}
	uploadsTestReset(t)
	flagApp = project
	original := apiBaseURL
	apiBaseURL = func() string { return endpoint }
	t.Cleanup(func() { apiBaseURL = original })
	files := map[string]string{
		"index.md":                "# Instructions\n",
		"references/a +#?&;雪%.md": "\ufeff# Exact Unicode 🌍\r\n",
		"100%.md":                 "", "literal%2F.md": "literal slash escape", "literal%25.md": "literal percent escape",
		"literal%252F.md": "double escape", "query?x=1&y=2#.md": "query characters",
		"quote\"'[]{}@!$().md": "punctuation", "日本語🌍.md": "Unicode name",
	}
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	skillsTestCommand(t, func() error { return skillsSetCommand(nil, []string{dir}) })
	listing := skillsTestCommand(t, func() error { return skillsListCommand(nil, nil) })
	for name, content := range files {
		if !strings.Contains(listing, name) {
			t.Errorf("listing omitted %q", name)
		}
		got := skillsTestCommand(t, func() error { return skillsGetCommand(nil, []string{name, "-"}) })
		if got != content {
			t.Errorf("%q: content changed: %q", name, got)
		}
	}
	renamed := "references/renamed +%2F雪.md"
	skillsTestCommand(t, func() error { return skillsRenameCommand(nil, []string{"literal%2F.md", renamed}) })
	files[renamed] = files["literal%2F.md"]
	delete(files, "literal%2F.md")
	archive := filepath.Join(t.TempDir(), "skills.zip")
	skillsTestCommand(t, func() error { return skillsBackupCommand(nil, []string{archive}) })
	flagSkillsForce = true
	skillsTestCommand(t, func() error { return skillsDeleteCommand(nil, []string{"all"}) })
	skillsTestCommand(t, func() error { return skillsRestoreCommand(nil, []string{archive}) })
	download := t.TempDir()
	skillsTestCommand(t, func() error { return skillsGetCommand(nil, []string{"all", download}) })
	for name, content := range files {
		got, err := os.ReadFile(filepath.Join(download, name))
		if err != nil || string(got) != content {
			t.Errorf("restored %q: %q, %v", name, got, err)
		}
	}
	skillsTestCommand(t, func() error { return skillsDeleteCommand(nil, []string{"all"}) })
}
