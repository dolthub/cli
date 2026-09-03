package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/jq"
	ghtemplate "github.com/cli/go-gh/v2/pkg/template"
	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

type Options struct {
	IO                                              *iostreams.IOStreams
	Config                                          func() (config.Config, error)
	APIClientForHost                                func(string) (*dolthub.Client, error)
	Endpoint, Method, Input, Hostname, JQ, Template string
	RawFields, TypedFields                          []string
	Paginate, Slurp, Include, Silent                bool
}

func NewCmdAPI(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, Config: f.Config, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = apiRun
	}
	c := &cobra.Command{Use: "api ENDPOINT", Short: "Make an authenticated DoltHub API request", Args: cmdutil.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error { o.Endpoint = a[0]; return runF(c.Context(), o) }}
	c.Flags().StringVarP(&o.Method, "method", "X", "", "HTTP method")
	c.Flags().StringArrayVarP(&o.RawFields, "raw-field", "f", nil, "Add a string parameter in key=value format")
	c.Flags().StringArrayVarP(&o.TypedFields, "field", "F", nil, "Add a typed parameter in key=value format")
	c.Flags().StringVar(&o.Input, "input", "", "File to use as request body (use - for stdin)")
	c.Flags().BoolVar(&o.Paginate, "paginate", false, "Make additional requests to fetch all pages")
	c.Flags().BoolVar(&o.Slurp, "slurp", false, "Wrap paginated JSON responses in an array")
	c.Flags().BoolVarP(&o.Include, "include", "i", false, "Include HTTP response status and headers")
	c.Flags().BoolVar(&o.Silent, "silent", false, "Do not print the response body")
	c.Flags().StringVar(&o.Hostname, "hostname", "", "DoltHub hostname")
	c.Flags().StringVarP(&o.JQ, "jq", "q", "", "Query to select values from the response using jq syntax")
	c.Flags().StringVarP(&o.Template, "template", "t", "", "Format JSON output using a Go template")
	c.PreRunE = func(*cobra.Command, []string) error {
		if o.Slurp && !o.Paginate {
			return cmdutil.FlagErrorf("--slurp requires --paginate")
		}
		if o.JQ != "" && o.Template != "" {
			return cmdutil.FlagErrorf("--jq and --template are mutually exclusive")
		}
		return nil
	}
	return c
}

func apiRun(ctx context.Context, o *Options) error {
	cfg, e := o.Config()
	if e != nil {
		return e
	}
	host := o.Hostname
	if host == "" {
		host = cfg.Host()
	}
	client, e := o.APIClientForHost(host)
	if e != nil {
		return e
	}
	body, e := requestBody(o)
	if e != nil {
		return e
	}
	method := strings.ToUpper(o.Method)
	if method == "" {
		method = http.MethodGet
		if len(body) > 0 {
			method = http.MethodPost
		}
	}
	endpoint := o.Endpoint
	pages := [][]byte{}
	seen := map[string]bool{}
	for {
		resp, e := client.Raw(ctx, method, endpoint, body)
		if e != nil {
			return e
		}
		if o.Include {
			fmt.Fprintf(o.IO.Out, "HTTP %d\n", resp.StatusCode)
			for k, v := range resp.Header {
				for _, x := range v {
					fmt.Fprintf(o.IO.Out, "%s: %s\n", k, x)
				}
			}
			fmt.Fprintln(o.IO.Out)
		}
		pages = append(pages, resp.Body)
		if !o.Paginate {
			break
		}
		next, e := nextToken(resp.Body)
		if e != nil {
			return e
		}
		if next == "" {
			break
		}
		if seen[next] {
			return fmt.Errorf("pagination token %q was repeated", next)
		}
		seen[next] = true
		endpoint = setPageToken(endpoint, next)
	}
	if o.Silent {
		return nil
	}
	payload := bytes.Join(pages, []byte("\n"))
	if o.Slurp {
		payload, e = json.Marshal(rawPages(pages))
		if e != nil {
			return e
		}
	}
	return formatOutput(o, payload)
}

func requestBody(o *Options) ([]byte, error) {
	if o.Input != "" && (len(o.RawFields) > 0 || len(o.TypedFields) > 0) {
		return nil, cmdutil.FlagErrorf("--input cannot be combined with fields")
	}
	if o.Input != "" {
		if o.Input == "-" {
			return io.ReadAll(o.IO.In)
		}
		return os.ReadFile(o.Input)
	}
	if len(o.RawFields)+len(o.TypedFields) == 0 {
		return nil, nil
	}
	m := map[string]any{}
	for _, f := range o.RawFields {
		k, v, e := field(f)
		if e != nil {
			return nil, e
		}
		m[k] = v
	}
	for _, f := range o.TypedFields {
		k, v, e := field(f)
		if e != nil {
			return nil, e
		}
		m[k] = typed(v)
	}
	return json.Marshal(m)
}
func field(v string) (string, string, error) {
	k, x, ok := strings.Cut(v, "=")
	if !ok || k == "" {
		return "", "", cmdutil.FlagErrorf("field must be in key=value format")
	}
	return k, x, nil
}
func typed(v string) any {
	if v == "true" {
		return true
	}
	if v == "false" {
		return false
	}
	if v == "null" {
		return nil
	}
	if i, e := strconv.ParseInt(v, 10, 64); e == nil {
		return i
	}
	return v
}
func nextToken(b []byte) (string, error) {
	var v struct {
		Meta dolthub.Meta `json:"meta"`
	}
	if e := json.Unmarshal(b, &v); e != nil {
		return "", fmt.Errorf("decode paginated response: %w", e)
	}
	return v.Meta.NextPageToken, nil
}
func setPageToken(endpoint, token string) string {
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("page_token", token)
	u.RawQuery = q.Encode()
	return u.String()
}
func rawPages(p [][]byte) []json.RawMessage {
	r := make([]json.RawMessage, len(p))
	for i := range p {
		r[i] = p[i]
	}
	return r
}
func formatOutput(o *Options, b []byte) error {
	r := bytes.NewReader(b)
	if o.JQ != "" {
		return jq.EvaluateFormatted(r, o.IO.Out, o.JQ, "", false)
	}
	if o.Template != "" {
		t := ghtemplate.New(o.IO.Out, 80, false)
		if e := t.Parse(o.Template); e != nil {
			return e
		}
		if e := t.Execute(r); e != nil {
			return e
		}
		return t.Flush()
	}
	if len(b) > 0 {
		_, e := o.IO.Out.Write(append(b, '\n'))
		return e
	}
	return nil
}
