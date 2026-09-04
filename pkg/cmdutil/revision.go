package cmdutil

import "github.com/dolthub/cli/internal/dolthub"

// RevisionSource validates and converts branch/commit source flags.
func RevisionSource(branch, commit string) (dolthub.RevisionSource, error) {
	if branch == "" && commit == "" {
		return dolthub.RevisionSource{}, FlagErrorf("exactly one of --from-branch or --from-commit is required")
	}
	if branch != "" && commit != "" {
		return dolthub.RevisionSource{}, FlagErrorf("--from-branch and --from-commit are mutually exclusive")
	}
	return dolthub.RevisionSource{Branch: branch, Commit: commit}, nil
}
