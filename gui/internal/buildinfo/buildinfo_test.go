package buildinfo

import "testing"

func TestGetReadsTheStamp(t *testing.T) {
	t.Cleanup(func() { repoCommit, repoBranch, repoTag, repoDirty, protoCommit = "", "", "", "", "" })
	repoCommit, repoBranch, repoTag, repoDirty = "abc123", "master", "v2.0.0", "true"
	protoCommit = "def456"

	got := Get()

	want := Git{Commit: "abc123", Branch: "master", Tag: "v2.0.0", Dirty: true}
	if got.Repo != want || got.Proto.Commit != "def456" || !got.Stamped || got.GoVersion == "" {
		t.Errorf("Get() = %+v, want Repo %+v, Proto.Commit def456, stamped", got, want)
	}
}

func TestGetWithoutAStampIsUnstamped(t *testing.T) {
	got := Get()

	// go test records no VCS state, so there's nothing to fall back to.
	if got.Stamped || got.Version != "dev" || got.Repo.Branch != "" || got.Proto != (Git{}) {
		t.Errorf("Get() = %+v, want an unstamped dev build", got)
	}
}
