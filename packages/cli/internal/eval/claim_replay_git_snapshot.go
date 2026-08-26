package eval

// GitClaimReplaySnapshot is one immutable knowledge-base archive from Git.
// Close removes its temporary files.
type GitClaimReplaySnapshot struct {
	Root      string
	Identity  string
	Reference string
	Commit    string
	close     func() error
}

func (snapshot GitClaimReplaySnapshot) Close() error {
	if snapshot.close == nil {
		return nil
	}
	return snapshot.close()
}

// CreateGitClaimReplaySnapshot extracts one revision without changing the
// caller's worktree.
func CreateGitClaimReplaySnapshot(root, revision string) (GitClaimReplaySnapshot, error) {
	snapshot, err := createBaseSnapshot(root, revision)
	if err != nil {
		return GitClaimReplaySnapshot{}, err
	}
	return GitClaimReplaySnapshot{
		Root: snapshot.root, Identity: snapshot.identity, Reference: snapshot.reference,
		Commit: snapshot.commit, close: snapshot.cleanup,
	}, nil
}
