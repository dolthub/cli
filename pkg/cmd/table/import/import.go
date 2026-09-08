package importcmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/operationwaiter"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/internal/upload"
	"github.com/dolthub/cli/pkg/cmd/operation/progress"
	viewcmd "github.com/dolthub/cli/pkg/cmd/operation/view"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

type apiClient interface {
	CreateImportUpload(context.Context, string, string, dolthub.CreateImportUploadRequest) (dolthub.ImportUpload, error)
	CreateImport(context.Context, string, string, dolthub.CreateImportRequest) (dolthub.OperationRef, error)
	GetOperation(context.Context, string) (dolthub.Operation, error)
	GetOperationURL(context.Context, string) (dolthub.Operation, error)
}

type Options struct {
	IO                                                 *iostreams.IOStreams
	ResolveRepository                                  func(context.Context, string) (repository.Repository, error)
	APIClientForHost                                   func(string) (*dolthub.Client, error)
	Repository, Branch, Table, File, FileType, Message string
	PrimaryKeys                                        []string
	Overwrite, Update, Replace, NoWait                 bool
	Exporter                                           cmdutil.Exporter
	client                                             apiClient
}

func NewCmdImport(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = importRun
	}
	c := &cobra.Command{
		Use: "import <table> <file>", Short: "Import a local data file into a DoltHub table",
		Long:    "Upload a CSV, PSV, XLSX, or JSON file and import it into a table.\nCreates a table by default. JSON requires --update or --replace.\nFiles must be regular, nonempty files of at most 1 GiB. Upload URLs expire\nafter 10 minutes; failed uploads must be restarted. Keep the file unchanged\nuntil uploading finishes. --no-wait still waits for the file upload.",
		Example: "  dh table import people people.csv --db owner/database --branch main --primary-key id",
		Args: func(c *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmdutil.FlagErrorf("%s requires a table and file", c.CommandPath())
			}
			o.Table, o.File = args[0], args[1]
			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error { return runF(c.Context(), o) },
	}
	cmdutil.AddDatabaseFlag(c, &o.Repository)
	c.Flags().StringVar(&o.Branch, "branch", "", "Target branch (required)")
	c.Flags().StringVar(&o.FileType, "file-type", "", "File format: csv, psv, xlsx, json (default: file extension)")
	c.Flags().StringSliceVar(&o.PrimaryKeys, "primary-key", []string{}, "Primary key columns (comma-separated or repeated)")
	c.Flags().StringVar(&o.Message, "message", "", "Import commit message")
	c.Flags().BoolVar(&o.Overwrite, "overwrite", false, "Overwrite an existing table")
	c.Flags().BoolVar(&o.Update, "update", false, "Update an existing table")
	c.Flags().BoolVar(&o.Replace, "replace", false, "Replace rows in an existing table")
	c.Flags().BoolVar(&o.NoWait, "no-wait", false, "Return after the import is accepted")
	cmdutil.AddJSONFlags(c, &o.Exporter, []string{"cancelable", "created_at", "error", "href", "id", "result", "status", "type"})
	return c
}

func importRun(ctx context.Context, o *Options) error {
	if strings.TrimSpace(o.Table) == "" || strings.TrimSpace(o.Branch) == "" {
		return cmdutil.FlagErrorf("a table and --branch are required")
	}
	op := dolthub.ImportCreate
	selected := 0
	for _, option := range []struct {
		enabled bool
		op      dolthub.ImportOperation
	}{{o.Overwrite, dolthub.ImportOverwrite}, {o.Update, dolthub.ImportUpdate}, {o.Replace, dolthub.ImportReplace}} {
		if option.enabled {
			selected++
			op = option.op
		}
	}
	if selected > 1 {
		return cmdutil.FlagErrorf("--overwrite, --update, and --replace are mutually exclusive")
	}
	typ := strings.ToLower(o.FileType)
	if typ == "" {
		typ = strings.TrimPrefix(strings.ToLower(filepath.Ext(o.File)), ".")
	}
	switch dolthub.ImportFileType(typ) {
	case dolthub.ImportCSV, dolthub.ImportPSV, dolthub.ImportXLSX, dolthub.ImportJSON:
	default:
		return cmdutil.FlagErrorf("unsupported file type %q; use --file-type csv, psv, xlsx, or json", typ)
	}
	if typ == "json" && (op == dolthub.ImportCreate || op == dolthub.ImportOverwrite) {
		return cmdutil.FlagErrorf("JSON imports require --update or --replace")
	}
	for _, key := range o.PrimaryKeys {
		if strings.TrimSpace(key) == "" {
			return cmdutil.FlagErrorf("primary key columns must not be empty")
		}
	}
	// Stat before opening so named pipes cannot block waiting for a writer.
	info, err := os.Stat(o.File)
	if err != nil {
		return fmt.Errorf("read import file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return cmdutil.FlagErrorf("import requires a regular file")
	}
	file, err := os.Open(o.File)
	if err != nil {
		return fmt.Errorf("open import file: %w", err)
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return cmdutil.FlagErrorf("import requires a regular file")
	}
	count, err := upload.NumParts(info.Size())
	if err != nil {
		return cmdutil.FlagErrorf("%s", err)
	}
	r, err := o.ResolveRepository(ctx, o.Repository)
	if err != nil {
		return err
	}
	c := o.client
	if c == nil {
		c, err = o.APIClientForHost(r.Host)
		if err != nil {
			return err
		}
	}
	session, err := c.CreateImportUpload(ctx, r.Owner, r.Name, dolthub.CreateImportUploadRequest{ContentLength: info.Size(), NumParts: count, FileType: dolthub.ImportFileType(typ)})
	if err != nil {
		return fmt.Errorf("create upload: %w", err)
	}
	var report func(int64)
	if o.IO.IsStderrTTY() {
		report = func(done int64) { _, _ = fmt.Fprintf(o.IO.ErrOut, "\rUploaded %d / %d bytes", done, info.Size()) }
	}
	transferred, err := upload.File(ctx, file, info.Size(), session, report)
	if report != nil {
		_, _ = fmt.Fprintln(o.IO.ErrOut)
	}
	if err != nil {
		return err
	}
	after, err := file.Stat()
	if err != nil {
		return err
	}
	if after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return fmt.Errorf("file changed during upload; rerun the command")
	}
	keys := o.PrimaryKeys
	if keys == nil {
		keys = []string{}
	}
	request := dolthub.CreateImportRequest{
		BranchName: o.Branch, TableName: o.Table, FileName: filepath.Base(o.File), FileSize: info.Size(), FileType: dolthub.ImportFileType(typ),
		ImportOperation: op, Token: session.Token, ContentsKey: session.ContentsKey, CompletedParts: transferred.Parts, FilePartsMD5: transferred.MD5, PrimaryKeys: keys,
	}
	if o.Message != "" {
		request.CommitMessage = &o.Message
	}
	ref, err := c.CreateImport(ctx, r.Owner, r.Name, request)
	if err != nil {
		return fmt.Errorf("submit import (check dh operation list before retrying): %w", err)
	}
	if o.NoWait {
		if o.Exporter != nil {
			return o.Exporter.Write(o.IO, ref)
		}
		t := tableprinter.New(o.IO, "ID", "HREF")
		_ = t.AddRow(ref.ID, ref.Href)
		return t.Render()
	}
	reporter := progress.New(o.IO, ref.ID)
	reporter.Start()
	defer reporter.Done()
	waiter := operationwaiter.Waiter{Client: c, Observe: reporter.Observe}
	operation, waitErr := waiter.Wait(ctx, ref)
	var renderErr error
	if o.Exporter != nil {
		renderErr = o.Exporter.Write(o.IO, operation)
	} else {
		renderErr = viewcmd.RenderHuman(o.IO, operation)
	}
	if renderErr != nil {
		return renderErr
	}
	return waitErr
}
