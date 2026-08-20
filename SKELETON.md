# `dh` CLI Skeleton Specification

## 1. Purpose

This document specifies a phased implementation of the initial skeleton for `dh`, a Go command-line client for DoltHub.

The skeleton should establish the architectural boundaries needed by future DoltHub commands without prematurely implementing product features. At the end of this work, contributors should be able to add a command by creating a focused package, injecting its dependencies through a factory, and testing it without accessing the network, the user's configuration, or a real Dolt repository.

This design borrows the strongest structural ideas from GitHub's `gh` CLI while intentionally omitting compatibility and ecosystem machinery that `dh` does not yet need.

## 2. Goals

The skeleton must provide:

- A Go module that builds a `dh` executable.
- A Cobra-based command tree with a small application lifecycle above it.
- Central dependency construction through a factory.
- A consistent `Options + NewCmd + run` pattern for leaf commands.
- Minimal configuration and authentication contracts.
- An authenticated DoltHub HTTP client with an injectable transport.
- A repository identity type and a deterministic base-repository resolver.
- A wrapper for invoking the local `dolt` executable.
- Testable terminal I/O, browser, and prompt boundaries.
- Central error-to-message and error-to-exit-code handling.
- A minimal machine-readable output abstraction.
- Unit tests that demonstrate how future commands should be built.

## 3. Non-goals

The initial skeleton will not implement:

- Full DoltHub feature coverage.
- A GraphQL client unless DoltHub APIs require one.
- OAuth device-code login; the initial login flow uses a web browser and a local callback or other documented browser completion mechanism.
- Multiple active accounts per host.
- Plugin or extension support.
- User-defined aliases.
- Telemetry.
- Automatic update checks.
- Shell completion generation beyond Cobra's default support.
- Pagers, spinners, alternate-screen rendering, or terminal theme detection.
- Configuration migrations beyond reserving a schema version.
- Enterprise feature detection.
- GitHub-specific fork-network or remote-resolution behavior.
- A general-purpose framework intended for use by other applications.

These features may be added later without changing the command construction pattern established here.

## 4. Architectural principles

### 4.1 Keep process concerns out of commands

The executable entry point should only call the application lifecycle and exit with its result. Configuration loading, dependency construction, root-command execution, error presentation, and exit-code selection belong in `internal/app`.

Leaf commands must return errors. They must not call `os.Exit`, log fatal errors, or independently decide process exit codes.

### 4.2 Inject capabilities, not global state

A central `cmdutil.Factory` supplies shared capabilities. Dependencies that may fail or may not be needed by every invocation should be exposed as lazy functions, for example `Config`, `HTTPClient`, and `BaseRepo`.

Commands should copy only the capabilities they require into their `Options` structures.

### 4.3 Keep interfaces small and consumer-oriented

Stable cross-cutting domain boundaries may be shared interfaces. Specialized dependencies should be expressed as small interfaces in the package that consumes them.

Do not introduce a single application-wide service interface.

### 4.4 Separate parsing from execution

Every non-trivial leaf command should follow this shape:

```go
type Options struct {
    IO       *iostreams.IOStreams
    Client   func() (*dolthub.Client, error)
    BaseRepo func() (repository.Repository, error)

    // Parsed arguments and flags follow.
}

func NewCmdFoo(f *cmdutil.Factory, runF func(*Options) error) *cobra.Command
func fooRun(opts *Options) error
```

`NewCmdFoo` owns Cobra metadata, arguments, flags, and flag validation. `fooRun` owns the operation. The optional `runF` is the constructor-test injection point.

### 4.5 Make external effects replaceable

Tests must be able to replace:

- HTTP through `http.RoundTripper`.
- Standard streams and TTY state through `IOStreams`.
- Configuration through interfaces or in-memory implementations.
- Repository resolution through a function.
- Browser and prompt behavior through small interfaces.
- Dolt command execution through a command runner boundary.
- Time through an injected function when output depends on it.

## 5. Proposed package layout

The initial target layout is:

```text
dolthub_cli/
├── cmd/
│   └── dh/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── app.go
│   │   └── app_test.go
│   ├── authflow/
│   │   ├── flow.go
│   │   └── flow_test.go
│   ├── browser/
│   │   └── browser.go
│   ├── config/
│   │   ├── config.go
│   │   ├── file.go
│   │   └── file_test.go
│   ├── credentials/
│   │   ├── store.go
│   │   └── store_test.go
│   ├── dolt/
│   │   ├── client.go
│   │   └── client_test.go
│   ├── dolthub/
│   │   ├── client.go
│   │   ├── errors.go
│   │   └── client_test.go
│   ├── prompt/
│   │   └── prompt.go
│   └── repository/
│       ├── repository.go
│       ├── resolve.go
│       └── resolve_test.go
├── pkg/
│   ├── cmd/
│   │   ├── auth/
│   │   │   └── auth.go
│   │   ├── factory/
│   │   │   └── default.go
│   │   ├── repo/
│   │   │   └── repo.go
│   │   ├── root/
│   │   │   ├── root.go
│   │   │   └── root_test.go
│   │   └── version/
│   │       ├── version.go
│   │       └── version_test.go
│   ├── cmdutil/
│   │   ├── errors.go
│   │   ├── factory.go
│   │   ├── output.go
│   │   ├── repo_override.go
│   │   └── repo_override_test.go
│   └── iostreams/
│       ├── iostreams.go
│       └── iostreams_test.go
├── test/
│   └── httpmock/
│       ├── registry.go
│       └── registry_test.go
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── SKELETON.md
```

Packages and files should only be created in the phase that needs them. Empty placeholder packages are discouraged.

The canonical module path is `github.com/dolthub/cli`, matching this repository's GitHub location.

## 6. Core contracts

The exact names may be refined during implementation, but changes should preserve these responsibility boundaries.

### 6.1 Dependency factory

```go
type Factory struct {
    AppVersion     string
    ExecutablePath string

    IO       *iostreams.IOStreams
    Browser  browser.Browser
    Prompter prompt.Prompter
    Dolt     dolt.Commander
    Credentials credentials.Store
    Authenticator authflow.Authenticator

    Config     func() (config.Config, error)
    HTTPClient func() (*http.Client, error)
    APIClient  func() (*dolthub.Client, error)
    BaseRepo   func() (repository.Repository, error)
}
```

Guidelines:

- The factory is a concrete wiring structure, not an interface.
- Production construction belongs in `pkg/cmd/factory`.
- Tests may instantiate `cmdutil.Factory` directly with only needed fields.
- Functions should be lazy where construction reads disk, inspects a repository, or may return an error.
- A command must not retain the entire factory inside its business logic when a narrower dependency will do.

### 6.2 Repository identity

Prefer a concrete immutable value unless polymorphism becomes necessary:

```go
type Repository struct {
    Host  string
    Owner string
    Name  string
}

func (r Repository) FullName() string
func Parse(value, defaultHost string) (Repository, error)
```

If later implementations require multiple repository representations, this may become an interface exposing `RepoHost`, `RepoOwner`, and `RepoName`. The skeleton should not introduce that indirection without a second implementation.

Accepted explicit forms should initially be:

- `OWNER/REPO`
- `HOST/OWNER/REPO`
- A recognized DoltHub repository URL

Parsing must reject ambiguous or malformed input with a flag/argument error suitable for showing usage.

### 6.3 Configuration

Keep the initial interface intentionally small:

```go
type Config interface {
    DefaultHost() string
    ActiveUser(host string) (string, bool)
    SetActiveUser(host, user string)
    DefaultRepository() (repository.Repository, bool)
    SetDefaultRepository(repository.Repository)
    Write() error
}
```

Tokens are secrets and should be accessed through a separate credential-store contract:

```go
type CredentialStore interface {
    Get(host, user string) (token string, err error)
    Set(host, user, token string) error
    Delete(host, user string) error
}
```

The concrete configuration implementation should use the platform-appropriate user config directory through `os.UserConfigDir`, with an overridable path for tests. The initial format may be YAML or JSON, but it must:

- Be documented.
- Contain host, active-user, and other non-secret metadata rather than tokens under normal operation.
- Be replaceable atomically rather than modified in place.
- Distinguish a missing config file from malformed configuration.
- Never print authentication tokens in errors or debug output.

The production credential store should use the operating system's secure credential store. If secure storage is unavailable, login must fail with an actionable error unless the user explicitly opts into a documented, restrictive-permission plaintext fallback. It must never silently downgrade token storage.

Environment-provided credentials should take precedence over stored credentials. The initial environment contract should reserve:

- `DH_TOKEN`
- `DH_HOST`
- `DH_REPO`

### 6.4 DoltHub API client

```go
type Client struct {
    httpClient *http.Client
    baseURL    *url.URL
}

func NewClient(httpClient *http.Client, baseURL *url.URL) *Client
```

The client should:

- Use an injected `*http.Client`.
- Attach authentication and user-agent headers through a transport wrapper constructed by the factory.
- Use request contexts supplied by callers.
- Decode successful JSON responses into typed models.
- Decode non-successful responses into a structured API error containing status, method, URL path, request ID when available, and a safe server message.
- Never include tokens or sensitive headers in errors.
- Keep endpoint-specific methods near their resource models.

The initial skeleton needs one harmless method, such as fetching the authenticated user or a health/version endpoint, only if a stable DoltHub endpoint is known. Otherwise, tests should exercise a package-private sample request path rather than inventing a public API contract.

### 6.5 Dolt command execution

The Dolt wrapper should separate process construction from higher-level repository inspection:

```go
type Commander interface {
    Run(ctx context.Context, args ...string) ([]byte, error)
}
```

The production implementation should:

- Resolve the `dolt` executable safely.
- Support a working-directory override.
- Capture stdout for parsing while routing or capturing stderr in a controlled way.
- Return a typed error containing the exit code and sanitized stderr.
- Use `exec.CommandContext` so cancellation terminates the child process.

Repository resolution should depend on `Commander`, not directly on `os/exec`.

### 6.6 I/O

The initial `IOStreams` should provide:

```go
type IOStreams struct {
    In     io.Reader
    Out    io.Writer
    ErrOut io.Writer
}
```

It should additionally expose TTY detection and test overrides for each stream. Color policy can be included if it remains small. Pager, spinner, and terminal-theme support are deferred.

A test constructor should return the streams plus readable input/output buffers.

### 6.7 Prompt and browser

Start with small interfaces:

```go
type Browser interface {
    Browse(url string) error
}

type Prompter interface {
    Input(label, defaultValue string) (string, error)
    Password(label string) (string, error)
    Confirm(label string, defaultValue bool) (bool, error)
}
```

Commands needing a smaller subset should declare a local interface rather than depend on the full shared interface.

### 6.8 Structured output

Reserve a minimal exporter contract:

```go
type Exporter interface {
    Write(io *iostreams.IOStreams, value any) error
}
```

Phase 1 needs only plain JSON output. Field selection, `jq`, and Go templates are deferred until a real list/view command establishes their requirements.

### 6.9 Semantic errors

Define errors that communicate presentation policy to the application lifecycle:

- `FlagError`: invalid arguments or flag combinations; print usage.
- `SilentError`: return failure without another message because the command already presented it.
- `CancelError`: user cancellation; use a distinct exit code.
- `NoResultsError`: a valid empty result; normally exit successfully.
- `AuthError`: missing or rejected authentication; provide actionable login guidance.
- `ExternalCommandError`: preserve a useful child-process exit code where appropriate.

Only `internal/app` maps these errors to exit codes and final messages.

Initial exit codes should be documented and stable:

| Code | Meaning |
|---:|---|
| 0 | Success, including a valid empty result |
| 1 | General failure |
| 2 | User cancellation or command-line usage error |
| 4 | Authentication required or rejected |

### 6.10 Browser authentication

Browser login is the first supported interactive authentication method. Its protocol-level implementation is intentionally deferred until the CLI's foundational code is in place and the DoltHub application registration details are available. The command-level contract should remain independent of the concrete OAuth or browser-completion protocol:

```go
type Authenticator interface {
    Login(ctx context.Context, host string) (LoginResult, error)
}

type LoginResult struct {
    Host     string
    Username string
    Token    string
}
```

The production implementation follows DoltHub's OAuth 2.0 authorization-code flow for a registered public client. Authorization uses `/oauth/authorize`; code and refresh-token exchanges use `/api/oauth/access_token` with form-encoded bodies. Public clients send a client ID but no client secret, use PKCE with `code_challenge_method=S256`, and request the `api_read_write` scope. The initial registered loopback redirect is `http://localhost:53682/callback`. Endpoint paths are resolved against the configured DoltHub web origin so the same contract works on the development site. The production public client ID must come from the approved `dh` registration; example or personal application IDs must not be embedded.

The expected user experience is:

```text
dh auth login
    -> determine and validate the DoltHub host
    -> create a short-lived browser authorization attempt
    -> open the authorization URL in the user's browser
    -> user authenticates with DoltHub or its configured identity provider
    -> dh receives or exchanges the successful result for a token
    -> dh validates the token and retrieves the authenticated username
    -> dh stores the token in the credential store
    -> dh records the active host and username in configuration
```

The flow must:

- Never ask for or receive the user's DoltHub password or external identity-provider password.
- Use a cryptographically random state value and verify it on callback when the protocol supports callbacks.
- Use PKCE when supported or required by the DoltHub authorization server.
- Bind a loopback callback listener only to loopback addresses and close it after success, cancellation, or timeout.
- Apply a finite timeout and honor context cancellation.
- Validate callback parameters and exchange the authorization result through an injected HTTP client.
- Validate the returned token by retrieving the authenticated identity before storing it.
- Avoid printing authorization codes, tokens, or credential-bearing URLs.
- Preserve a manually copyable authorization URL if opening the browser fails.
- Store credentials only after the entire authentication and identity-validation flow succeeds.

Browser opening, callback handling, token exchange, identity lookup, configuration, and credential storage must each be replaceable in tests. Tests must never open a real browser or bind a fixed port.

## 7. Repository resolution policy

Repository resolution is a core `dh` behavior and must be centralized. The initial precedence is:

1. `--repo` on the current command or an ancestor.
2. `DH_REPO`.
3. A repository recorded as the configured default.
4. A recognized remote in the current Dolt repository.
5. An actionable error explaining how to use `--repo` or configure a default.

The remote resolution phase should begin only after the relevant Dolt CLI output and DoltHub URL formats have been verified. It must not assume that GitHub remote rules apply to Dolt.

The resolver should return a normalized `repository.Repository`; commands should not know which source won.

`EnableRepoOverride` should install the persistent `--repo` flag on repository-oriented parent commands and replace the factory's `BaseRepo` function before leaf execution. Authentication checks must remain effective when this hook is installed.

## 8. Phased implementation plan

### Phase 0: Decisions and repository baseline

Objective: remove foundational ambiguity before generating code.

Deliverables:

- Confirm the canonical Go module path.
- Confirm the minimum supported Go version.
- Confirm the default DoltHub API and web hostnames.
- Confirm and document DoltHub's browser authorization protocol, callback/completion mechanism, client registration, token exchange, identity endpoint, timeout rules, and supported redirect URIs.
- Confirm whether PKCE is required and whether a client secret is appropriate for an installed public CLI client.
- Confirm the initial configuration format and path.
- Document the supported operating systems.
- Add a concise `README.md` describing the project status and local build command.
- Add a `.gitignore` for build and test artifacts if the existing file is insufficient.

Acceptance criteria:

- Contributors can identify the module path, toolchain version, default host, and config location without reading source code.
- No public API endpoint or authentication behavior is guessed. Unknowns are recorded explicitly.
- The browser login protocol can be implemented from authoritative DoltHub contracts without scraping pages or handling a user's password.

Recorded decisions:

- Module path: `github.com/dolthub/cli`.
- Minimum Go version: Go 1.26.0.
- Supported operating systems: Linux, macOS, and Windows.
- DoltHub web origin: `https://www.dolthub.com`.
- DoltHub REST API: v2 at `https://www.dolthub.com/api/v2/`. The published OpenAPI 3.1 specification is the source of truth for endpoints and models.
- Configuration format: JSON with a reserved schema-version field.
- Configuration path: `dh/config.json` beneath `os.UserConfigDir()` (for example, `$XDG_CONFIG_HOME/dh/config.json` on Linux when set). Tests and callers may override the path.
- Browser login uses the public-client authorization-code contract recorded in Section 6.10. The approved production `dh` client ID is supplied when the production authenticator is wired; personal or example registrations are not used.

### Phase 1: Buildable executable and root command

Objective: establish the process and command-tree boundaries.

Deliverables:

- Initialize `go.mod` with Cobra as the only required CLI framework dependency.
- Add `cmd/dh/main.go`.
- Add `internal/app.Main(args, stdin, stdout, stderr) int` or an equivalently testable signature.
- Add `pkg/cmd/root.NewCmdRoot`.
- Add `dh version` using build-time version variables with a development fallback.
- Add root help, usage, and unknown-command behavior.
- Add a `Makefile` with at least `build`, `test`, and `fmt` targets.

Design requirements:

- Tests must call the application without modifying global `os.Args` or global standard streams.
- `main.go` should contain no application logic beyond passing process values to `app.Main` and calling `os.Exit`.
- Root construction must not require valid credentials or a repository.

Acceptance criteria:

- `go test ./...` passes.
- `go build ./cmd/dh` succeeds.
- `dh --help` identifies the executable as the DoltHub CLI.
- `dh version` succeeds without configuration, network access, or a Dolt repository.
- An unknown command produces a non-zero exit and useful usage output.

### Phase 2: I/O and semantic error handling

Objective: standardize command output and process outcomes before adding effects.

Deliverables:

- Add `pkg/iostreams` with system and test constructors.
- Add TTY detection with deterministic test overrides.
- Add the semantic errors described above.
- Centralize error rendering and exit-code mapping in `internal/app`.
- Add helpers for flag-validation errors.

Acceptance criteria:

- Unit tests cover success, general failure, flag error, cancellation, authentication error, silent error, and no-results behavior.
- Leaf commands do not print returned errors a second time.
- Usage is printed for argument/flag errors but not ordinary API failures.
- Tests can independently capture stdout and stderr.

### Phase 3: Factory and canonical command pattern

Objective: establish dependency injection and demonstrate the required command shape.

Deliverables:

- Add `pkg/cmdutil.Factory` with only currently implemented dependencies.
- Add `pkg/cmd/factory.New` for production wiring.
- Refactor root construction to accept the factory.
- Implement one harmless demonstration command, preferably `dh config get` or a hidden/internal diagnostic command, using `Options + NewCmd + run`.
- Add one constructor test that injects `runF` and verifies parsed options.
- Add one run-function test using fake dependencies.

Acceptance criteria:

- The example command has no direct use of global standard streams, global arguments, or process exit.
- Constructor tests do not execute its business behavior.
- Run-function tests do not require Cobra parsing.
- A future contributor can copy this command as the canonical template.

### Phase 4: Configuration and secure credential storage

Objective: provide safe, testable local state for the browser login flow.

Deliverables:

- Add the minimal `config.Config` contract and disk-backed implementation.
- Add the `CredentialStore` contract and an operating-system credential-store implementation.
- Implement environment precedence for `DH_HOST`, `DH_TOKEN`, and `DH_REPO` where applicable.
- Implement atomic writes for non-secret configuration.
- If plaintext token storage is supported, put it behind an explicit opt-in and use restrictive file permissions.
- Add `dh config get` and `dh config set` only for explicitly supported non-secret settings.
- Add in-memory configuration and credential-store fakes for tests.

Security requirements:

- Tokens must never appear in normal output, errors, test snapshots, or HTTP debug logs.
- Configuration tests must use a temporary directory and path injection.
- Malformed configuration must return an actionable error and must not be silently overwritten.
- Secure-store failure must not silently downgrade to plaintext storage.

Acceptance criteria:

- A missing configuration file behaves as an empty/default configuration.
- A malformed configuration file fails safely.
- Environment values override persisted values.
- Writes preserve unrelated supported settings.
- Credential-store get, set, delete, missing-secret, and failure behavior are tested.
- File permission behavior for any explicitly enabled plaintext fallback is tested on platforms where it is meaningful.

### Phase 5: HTTP transport and DoltHub API client

Objective: establish one secure, typed path for all DoltHub network access.

Deliverables:

- Add authenticated and unauthenticated HTTP transport construction.
- Add a user-agent containing the `dh` version.
- Add the DoltHub client and structured API error.
- Add request-context support.
- Add a transport-level HTTP mock registry or use a small existing test library if it reduces maintenance without hiding request behavior.
- Add tests for URL construction, authentication headers, JSON decoding, error decoding, cancellation, and token redaction.
- Add the verified DoltHub authorization, token-exchange, and authenticated-identity API operations required by browser login.

Design requirements:

- The API client accepts `*http.Client`; it does not construct the default client internally.
- Authentication is added in one transport layer rather than repeated in endpoint methods.
- API methods receive `context.Context`.
- Endpoint paths are relative to a configured base URL so tests can use `httptest.Server`.

Acceptance criteria:

- Tests make no external network requests.
- An expected request that is not made fails its test.
- An unexpected request fails immediately.
- HTTP errors retain status and safe server context.
- Neither request nor error formatting exposes authorization headers.

### Phase 6: First vertical command slice - browser authentication

Objective: deliver `dh auth login` as the first complete user-facing feature, with status and logout lifecycle support.

Deliverables:

- Add the `dh auth` command group with authentication checks disabled for its subcommands.
- Implement `dh auth login` using the browser authentication contract in Section 6.10.
- Open the browser through the injected `browser.Browser` interface and print a safe manual URL when automatic opening fails.
- Receive and validate browser completion according to DoltHub's documented protocol.
- Exchange the authorization result for a token when required.
- Retrieve the authenticated DoltHub username before persisting anything.
- Store the token through `CredentialStore` and record the active host/user in configuration.
- Implement `dh auth status` to validate and report the active host and user without displaying the token.
- Implement `dh auth logout` to remove the stored token and active account metadata, with confirmation when interactive.
- Recognize `DH_TOKEN` as a non-persisted automation credential and clearly identify its source in `auth status`.
- Refuse to overwrite or delete an environment-provided token because it is outside `dh`'s credential store.
- Add constructor tests, run-function tests, callback/protocol tests, fake-browser tests, HTTP tests, storage-failure tests, cancellation tests, and application-level exit-code tests.

Acceptance criteria:

- `dh auth login` opens or presents the correct DoltHub authorization URL.
- A completed browser login stores only the validated resulting token, never a user password or IdP credential.
- State mismatch, rejected authorization, timeout, cancellation, malformed callback, exchange failure, and identity-validation failure store no credentials.
- Successful login stores the token securely and records the matching host and username.
- Re-login has an explicit, tested replacement policy and cannot accidentally replace credentials for a different host.
- `dh auth status` distinguishes stored credentials, environment credentials, invalid credentials, and no credentials.
- `dh auth logout` removes credentials owned by `dh` and leaves environment credentials untouched.
- All tests are hermetic: they open no real browser, contact no external server, use no real credential store, and bind no fixed network port.
- Tokens and authorization secrets are absent from all output and errors.

### Phase 7: Dolt process wrapper

Objective: make local Dolt inspection and future mutation testable and cancellable.

Deliverables:

- Add `dolt.Commander` and its production implementation.
- Support working-directory selection.
- Add a typed command error with exit code and sanitized stderr.
- Add tests using a fake executable or injected process-construction function.
- Add narrowly scoped helpers only when required by repository resolution, such as reading remotes.

Acceptance criteria:

- A missing `dolt` executable produces an actionable error.
- Context cancellation terminates or returns from an in-flight command.
- Argument passing is not performed through a shell.
- Tests cover success, non-zero exit, missing executable, and malformed output.

### Phase 8: Repository parsing and base-repository resolution

Objective: give all repository-aware commands one deterministic source of repository context.

Deliverables:

- Add the repository value type, normalization, parsing, and formatting.
- Implement explicit `--repo` parsing.
- Implement `DH_REPO` and configured-default resolution.
- After verifying Dolt output formats, implement current-directory remote resolution.
- Add `cmdutil.EnableRepoOverride` for repository command groups.
- Wire `Factory.BaseRepo` lazily.
- Add table-driven resolver tests covering every source and precedence combination.

Acceptance criteria:

- Explicit `--repo` works outside a Dolt repository.
- `--repo` takes precedence over all implicit sources.
- Commands receive the same normalized repository representation regardless of source.
- Malformed explicit input is reported as a usage error.
- Missing implicit context produces instructions for both `--repo` and configuration.
- Resolver tests never invoke a real Dolt command.

### Phase 9: First repository command slice

Objective: prove that the authenticated foundation composes cleanly with repository context and a real, read-only DoltHub operation.

The preferred first slice is `dh repo view [OWNER/REPO]`, subject to confirmation of the DoltHub API. It should exercise repository resolution, authenticated or public API access, human output, JSON output, browser behavior, and error handling.

Deliverables:

- Add the `repo` command group.
- Add `repo view` with positional and `--repo` resolution.
- Add `--web` through `browser.Browser`.
- Add `--json` using the minimal exporter.
- Define typed API response models for only the fields used.
- Add constructor, run-function, HTTP, TTY/non-TTY, JSON, web, authentication, and no-results tests.

Acceptance criteria:

- The command works with an explicit repository outside a local repository.
- Human-readable output is stable and tested separately for TTY and non-TTY modes if they differ.
- JSON output contains no decorative text.
- `--web` does not make an unnecessary API request unless URL resolution requires it.
- Tests validate outgoing method, path, headers, and relevant query parameters.
- This command serves as the documented template for subsequent API commands.

### Phase 10: Documentation and contributor guardrails

Objective: make the skeleton sustainable for parallel feature development.

Deliverables:

- Document the package layout and execution flow in `README.md` or `docs/architecture.md`.
- Document how to add a command.
- Document test patterns for constructors, run functions, HTTP, config, and Dolt execution.
- Add formatting, vetting, and test checks to CI.
- Add a linter configuration only after agreeing on rules; do not import a large inherited ruleset blindly.
- Add contribution guidance for handling secrets and security reports.

Acceptance criteria:

- A new contributor can add a read-only command without changing application startup or production factory internals unless it needs a genuinely new capability.
- CI builds the executable and runs all tests from a clean checkout.
- Documentation examples match the current source tree.

## 9. Testing strategy

### 9.1 Command constructor tests

Each leaf command should have table-driven tests that:

- Construct a minimal factory.
- Inject `runF` to capture options.
- Execute the Cobra command with representative arguments.
- Verify defaults, parsed flags, validation, and mutually exclusive options.
- Avoid network, filesystem, and subprocess effects.

### 9.2 Run-function tests

Run tests should invoke the run function directly with:

- Test IO streams.
- Fixed repository values.
- Fake browser/prompter implementations.
- Mock HTTP transports.
- Fake Dolt commanders.
- Fixed clocks where applicable.

They should verify output, returned semantic errors, and external interactions.

### 9.3 HTTP client tests

HTTP tests should validate:

- Method and resolved URL.
- Required headers and absence of inappropriate headers.
- Request bodies.
- Success decoding.
- Structured error decoding.
- Empty and malformed responses.
- Cancellation.
- Redaction of tokens and other secrets.

### 9.4 Application tests

Application-level tests should cover:

- Root help and version.
- Unknown commands.
- Exit-code mapping.
- Stdout/stderr separation.
- Operation without a config file.

They should call an injectable `app.Main` rather than launching the compiled binary except for a small number of integration smoke tests.

### 9.5 Integration tests

Initially limit integration tests to:

- Building and invoking `dh version`.
- Invoking `dh --help`.
- A fake-server and fake-credential-store invocation of the browser authentication slice.
- A fake-server invocation of the first repository command.

Tests requiring a real DoltHub account or mutating a real repository should not be part of the default suite.

## 10. Dependency policy

Prefer the Go standard library except where a focused dependency provides clear value.

Expected initial dependencies:

- `github.com/spf13/cobra` for command parsing.
- A small YAML library only if YAML is selected for configuration.
- `github.com/stretchr/testify` is optional; standard-library tests are acceptable if consistency is maintained.

Do not add a dependency-injection framework. Do not add a full HTTP SDK until the required API surface is understood. Every dependency should be justified by current code rather than anticipated scale.

## 11. Security baseline

The skeleton must establish these rules before authentication or mutation commands are added:

- Never log or format bearer tokens, authorization headers, passwords, or credential-bearing URLs.
- Do not pass untrusted arguments through a shell.
- Use context-aware HTTP requests and subprocesses.
- Sanitize API messages before rendering them to an interactive terminal if they can contain control sequences.
- Store tokens in the operating system credential store; any plaintext fallback must be explicit, documented, and protected with the narrowest practical file permissions.
- Validate hostnames and URL schemes before attaching credentials.
- Attach DoltHub credentials only to configured, trusted DoltHub API hosts.
- Tests must use unmistakably fake tokens and must assert redaction.

## 12. Definition of skeleton complete

The foundational skeleton is complete when Phases 0 through 5 are satisfied. Browser authentication is intentionally the first feature built on that skeleton in Phase 6; Dolt and repository functionality follows only after the login lifecycle is complete. The initial project roadmap in this document is complete when Phases 0 through 10 are satisfied.

At that point:

- `dh` builds and has stable help/version behavior.
- Process lifecycle and exit behavior are centralized.
- Commands follow the documented constructor/run pattern.
- Dependencies are supplied through a small factory.
- Browser login, token validation, secure storage, status, and logout are implemented and independently testable.
- Configuration, authentication lookup, API transport, Dolt execution, and repository resolution are independently testable.
- Authentication and repository commands exercise their respective full paths without architectural exceptions.
- All default tests are hermetic and require neither network access nor user configuration.

Feature development beyond that point should proceed as vertical slices. Each new command should add only the domain models, API methods, and local capabilities it actually requires.
