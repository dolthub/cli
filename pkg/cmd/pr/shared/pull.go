package shared

import (
	"strconv"

	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

var JSONFields = []string{"created_at", "creator", "description", "from_branch", "pull_number", "state", "title", "to_branch"}

func ParseNumber(value string) (int64, error) {
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number < 1 {
		return 0, cmdutil.FlagErrorf("pull request number must be a positive integer")
	}
	return number, nil
}

func RenderPull(streams *iostreams.IOStreams, pull dolthub.Pull) error {
	t := tableprinter.New(streams, "NUMBER", "TITLE", "STATE", "FROM", "INTO")
	_ = t.AddRow(strconv.FormatInt(pull.PullNumber, 10), pull.Title, string(pull.State), Branch(pull.FromBranch), Branch(pull.ToBranch))
	return t.Render()
}

func Branch(ref dolthub.BranchRef) string {
	return ref.Database.Owner + "/" + ref.Database.Name + ":" + ref.BranchName
}
