package config

import "testing"

func stubGitRemote(t *testing.T, url string, err error) {
	t.Helper()
	prev := gitRemoteGetter
	gitRemoteGetter = func(repoRoot string) (string, error) {
		return url, err
	}
	t.Cleanup(func() {
		gitRemoteGetter = prev
	})
}
