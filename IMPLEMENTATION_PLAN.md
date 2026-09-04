# `dh` phased implementation plan

Status: active — Phases 0–6 merged; Phase 7 is next

This plan implements the surface in [COMMANDS.md](./COMMANDS.md) from easiest
to hardest. It is organized around small, reviewable pull requests and the
current DoltHub REST API v2 contract.

## Branch and pull request policy

Every user-visible command is implemented on its own branch and in its own pull
request. In this document, "command" means the executable leaf command, such as
`dh db view` or `dh pr list`, not merely a parent command group.

Rules for every command PR:

1. The branch name is the command path, for example `db/view`, `pr/list`, or
   `completion`. Foundation branches use `core/<capability>`.
2. A command PR contains the Cobra constructor, options and run logic, API
   method calls, rendering, help examples, and tests for exactly one leaf
   command. Creating its otherwise-empty parent group is allowed in the first
   child PR for that group.
3. Shared infrastructure needed by multiple commands goes in a preceding
   `core/*` PR. It must not silently add a user-visible command.
4. Command registration happens in the same PR as the command. The root help
   must never advertise an unimplemented command.
5. Each PR must pass `go test ./...`, `go vet ./...`, `git diff --check`, and
   command-specific tests. Tests which bind a loopback listener may need to run
   outside a restricted sandbox.
6. Each command PR includes constructor/flag tests, run-function tests, mocked
   HTTP tests when applicable, TTY and non-TTY output tests, and JSON output
   tests for resource-producing commands.
7. PR descriptions identify the OpenAPI operation IDs used and link to the PR
   immediately below them when stacked.
8. No command PR uses private gRPC methods or legacy HTTP APIs to fill a v2
   contract gap.

Use short stacks within a phase, ordered by dependency. Do not build one stack
containing the whole roadmap. A typical phase is:

```text
main
  core/operation-waiter
    operation/watch
      db/fork
        pr/merge
```

Each branch maps to one PR whose base is the branch below it. Submit stacks
non-interactively with `gh stack submit --auto`, inspect them with
`gh stack view --json`, and merge or restack the phase before starting a stack
for the next phase. Independent command branches may instead target `main`
directly after their foundation PRs merge.

## Definition of done

A command is complete when:

- Its supported behavior and flags agree with `COMMANDS.md`.
- Every API call uses a typed client method, except `dh api` by design and
  direct object-storage PUTs during `dh db import`.
- Public reads work anonymously and authenticated reads also work for private
  databases.
- Errors preserve the v2 problem status, code, request ID, method, and path
  without exposing credentials or unsafe server text.
- List commands correctly follow opaque `meta.next_page_token` cursors and
  honor `--limit`.
- Human output is useful in a terminal and stable in a pipe.
- Applicable resources support `--json`, `--jq`, and `--template`.
- Help and README examples are updated in the command's PR.

## Phase 0: foundations

Status: complete. The five-PR stack was merged on 2026-09-03:

| Order | Branch | Pull request |
| ---: | --- | --- |
| 0.1 | `core/api-client` | [#14](https://github.com/dolthub/cli/pull/14) |
| 0.2 | `core/api-models` | [#15](https://github.com/dolthub/cli/pull/15) |
| 0.3 | `core/database-resolver` | [#16](https://github.com/dolthub/cli/pull/16) |
| 0.4 | `core/output` | [#17](https://github.com/dolthub/cli/pull/17) |
| 0.5 | `core/pagination` | [#18](https://github.com/dolthub/cli/pull/18) |

These are internal capability PRs, not command PRs. They are ordered so each
one is independently testable and leaves the existing command surface working.

| Order | Branch | Scope |
| ---: | --- | --- |
| 0.1 | `core/api-client` | Add generic request construction, POST/PATCH JSON bodies, typed success-envelope decoding, pagination metadata, response limits, safe path escaping, and same-origin URL validation. Preserve `CurrentUser`. |
| 0.2 | `core/api-models` | Add v2 DTOs and request types for Database, Branch, Tag, Release, Pull, PullComment, QueryResult, Operation, ImportUpload, and their request bodies. Keep JSON field names aligned with OpenAPI. |
| 0.3 | `core/database-resolver` | Resolve `[HOST/]OWNER/DB` from `-R/--db`, `DH_REPO`, config, and recognized local Dolt remotes. Keep resolution lazy and injectable. Do not add a `db` command yet. |
| 0.4 | `core/output` | Add table rendering and `gh`-style `--json`, `--jq`, and `--template` support with explicit per-resource field lists. |
| 0.5 | `core/pagination` | Add a cursor pager which treats page tokens as opaque, stops at `--limit`, handles cancellation, and detects a repeated token. |

Phase 0 exit criterion: a command can resolve a database, call any synchronous
v2 JSON operation, paginate a list, and render human or structured output.

## Phase 1: local and bounded read commands

Status: complete. The five-PR stack was merged on 2026-09-03:

| Order | Branch | Pull request |
| ---: | --- | --- |
| 1.1 | `completion` | [#20](https://github.com/dolthub/cli/pull/20) |
| 1.2 | `config/list` | [#21](https://github.com/dolthub/cli/pull/21) |
| 1.3 | `db/view` | [#22](https://github.com/dolthub/cli/pull/22) |
| 1.4 | `browse` | [#23](https://github.com/dolthub/cli/pull/23) |
| 1.5 | `operation/view` | [#24](https://github.com/dolthub/cli/pull/24) |

These have no mutation semantics and require no pagination.
Most make no HTTP request or one simple GET; `db view --forks` makes one
additional bounded request. They establish command conventions at low risk.

Build Phase 1 as one stack, with one command per branch:

```text
main
  completion
    config/list
      db/view
        browse
          operation/view
```

| Order | Branch / PR | Command | API operations | Why here |
| ---: | --- | --- | --- | --- |
| 1.1 | `completion` | `dh completion` | None | Cobra generates completions locally; smallest new command. |
| 1.2 | `config/list` | `dh config list` | None | Exercises stable non-secret tabular output without API behavior. |
| 1.3 | `db/view` | `dh db view [DB]` | `getDatabase`, optionally `listForks` | First database-scoped typed read and first use of resolver/output foundations. |
| 1.4 | `browse` | `dh browse` | None | Reuses database resolution and existing browser abstraction; URL-only behavior. |
| 1.5 | `operation/view` | `dh operation view ID` | `getOperation` | One authenticated GET and important validation for opaque/slash-containing operation IDs. |

`db/view` creates and registers the `db` parent group. `operation/view` creates
and registers the `operation` parent group.

### Phase 1.1: `completion`

- Accept exactly one positional shell: `bash`, `fish`, `powershell`, or `zsh`.
- Generate completion from the fully registered root command and write only the
  script to stdout. Do not load config, credentials, or an HTTP client.
- Reject a missing or unsupported shell as a usage error.
- Test every shell, argument validation, registration in root help, and that
  generated output is non-empty.

### Phase 1.2: `config list`

- Print every supported non-secret key: initially `host` and `repo`.
- Show columns `KEY`, `VALUE`, and `SOURCE`. Source is one of `environment`,
  `config`, `default`, or `unset`; an unset repo has an empty value.
- Environment values take precedence over persisted values. Invalid `DH_REPO`
  is an error rather than silently falling through to persisted config.
- Use headers and aligned columns on a terminal; use stable tab-separated rows
  without headers otherwise. This command does not support structured output
  because it exists to inspect configuration provenance rather than a v2
  resource.
- Test all four source labels, environment precedence, invalid environment
  input, TTY output, non-TTY output, and secret redaction by construction.

### Phase 1.3: `db view`

- Accept at most one positional `[HOST/]OWNER/DATABASE` and `-R/--db` as an
  alternative. Reject using both. With neither, use the shared resolver.
- `--web` opens the database root URL and makes no API request. It is mutually
  exclusive with `--forks`, `--json`, `--jq`, and `--template`.
- Normal mode calls `getDatabase`. `--forks` additionally calls `listForks`
  and includes immediate children only.
- Human output includes owner/name, visibility, description, size, stars,
  last-write time, parent, network root, and fork-network count. An absent
  optional value is rendered as `-` rather than inferred.
- Structured fields use the API v2 snake_case names: `owner`, `name`,
  `description`, `visibility`, `fork_network_count`, `star_count`,
  `size_bytes`, `last_write_at`, `parent`, and `network_root`; `forks` is
  available only with `--forks`.
- Public databases must work anonymously. Test escaped owner/database path
  segments, private authenticated reads, RFC 9457 errors, TTY and non-TTY
  rendering, every structured field, `--forks`, and incompatible flags.

### Phase 1.4: `browse`

- Accept no positional argument or one positive pull-request number. Also
  accept `--pull NUMBER` and `--branch NAME`; these target selectors are
  mutually exclusive.
- Resolve the database exactly as other database-scoped commands do, including
  `-R/--db`. Make no API request.
- Construct only these URL shapes, escaping every dynamic path segment:
  - database: `/repositories/{owner}/{database}`
  - pull request: `/repositories/{owner}/{database}/pulls/{number}`
  - branch data: `/repositories/{owner}/{database}/data/{branch}`
- Do not accept an arbitrary path or guess whether a string is a branch, tag,
  commit, table, or document. Add explicit selectors later when their URL
  semantics are specified.
- Test every URL shape, custom hosts through resolver injection, escaping,
  selector conflicts, invalid pull numbers, browser failures, and the absence
  of API calls.

### Phase 1.5: `operation view`

- Accept exactly one opaque operation ID and call authenticated
  `getOperation`. Select the API origin from effective host configuration; this
  command is not database-scoped.
- Escape the ID as one path segment even when it contains slashes. Do not split
  it, parse it, or construct a database from its contents.
- Human output includes ID, type, status, created time, cancelable state, and
  any error or result. Structured fields are `id`, `type`, `status`,
  `created_at`, `cancelable`, `error`, and `result`.
- Viewing an operation whose status is `failed` succeeds after displaying its
  recorded error. `operation watch`, implemented later, is responsible for a
  nonzero exit when a watched operation terminates unsuccessfully.
- Test authentication requirements, slash-containing IDs, each operation
  status, failed-operation rendering, dynamic result JSON, RFC 9457 errors,
  TTY and non-TTY rendering, and every structured field.

Phase 1 exit criterion: all five commands are registered only on their own
branches, public database reads work without login, authenticated reads work
with stored or environment credentials, browser commands make no API calls,
and the full stack passes the cross-platform test and lint workflows.

## Phase 2: list commands

Status: complete. The four-PR stack was merged on 2026-09-03:

```text
main
  api
    release/list
      pr/list
        operation/list
```

`dh api` provides the same low-level escape hatch as `gh api`, including access
to branch, tag, and immediate-fork list endpoints. Typed list commands are
limited to commands with direct `gh` equivalents.

| Order | Branch / PR | Command | API operations | Notes |
| ---: | --- | --- | --- | --- |
| 2.1 | [`api` / #33](https://github.com/dolthub/cli/pull/33) | `dh api ENDPOINT` | Generic v2 access | Direct counterpart to `gh api`; exposes endpoints without inventing typed commands. |
| 2.2 | [`release/list` / #34](https://github.com/dolthub/cli/pull/34) | `dh release list` | `listReleases` | Direct counterpart to `gh release list`. |
| 2.3 | [`pr/list` / #35](https://github.com/dolthub/cli/pull/35) | `dh pr list` | `listPulls` | Direct counterpart to `gh pr list`; state filtering is client-side. |
| 2.4 | [`operation/list` / #36](https://github.com/dolthub/cli/pull/36) | `dh operation list` | `listOperations` | Dolt operation counterpart to `gh run list`. |

The three typed commands are database-scoped and accept `-R/--db`.
Paginated commands accept `--limit N`, defaulting to 30, and reject values below 1 as a
usage error. They preserve backend order, pass `meta.next_page_token` back as
an opaque `page_token`, stop without another request once the limit is met,
and fail if a token repeats. An empty successful list exits zero and prints no
rows. Public databases work anonymously; a configured credential is used when
available for private databases.

Human output uses uppercase aligned headers on a terminal and stable
tab-separated rows without headers when piped. Times use RFC 3339 when present
and `-` when absent. Structured output operates on the final collected list,
supports `--json`, `--jq`, and `--template`, and uses the API's snake_case
field names.

### Phase 2.1: `api`

- Implement the `dh api` contract in `COMMANDS.md`, including methods, typed
  fields, request bodies, response headers, silent mode, jq/templates, and v2
  envelope pagination.
- Relative endpoints resolve beneath `/api/v2/`; absolute URLs and paths that
  escape that prefix are rejected before credentials can be attached.
- Branches, tags, and immediate forks remain available through this command,
  for example `dh api databases/OWNER/DATABASE/branches --paginate`.
- Test method inference, field encoding, stdin/file bodies, same-origin safety,
  anonymous and authenticated requests, headers, pagination, slurp, filters,
  errors, and cancellation.

### Phase 2.2: `release list`

- Create and register the `release` parent group.
- Call `listReleases` and paginate to `--limit`.
- Human columns are `TAG`, `TITLE`, `COMMIT`, `CREATED`, and `UPDATED`.
  Description remains available in structured output rather than expanding
  multiline markdown inside a table.
- Structured fields are `tag`, `title`, `commit_sha`, `description`,
  `created_at`, and `updated_at`.
- Test multiline descriptions, pagination, limit boundaries, empty results,
  table modes, structured output, and API errors.

### Phase 2.3: `pr list`

- Create and register the `pr` parent group.
- Accept `--state open|closed|merged|all`, defaulting to `open`. Reject any
  other value as a usage error.
- Because `listPulls` has no filter parameter, retain backend order while
  filtering each page locally. Continue fetching until `--limit` matching
  items have been collected or pagination ends; nonmatching items do not count
  toward the limit.
- Human columns are `NUMBER`, `TITLE`, `STATE`, `CREATOR`, and `CREATED`.
- Structured fields are `pull_number`, `title`, `description`, `state`,
  `created_at`, and `creator`.
- Test every state, the default, `all`, pages containing only nonmatching
  items, filtered limit boundaries, repeated tokens, empty results, table
  modes, structured output, and API errors.

### Phase 2.4: `operation list`

- Extend the existing `operation` parent group; unlike `operation view`, this
  command is database-scoped and authentication is optional.
- Call `listOperations` for the resolved repository and paginate to `--limit`.
  Do not add client-side type or status filters in this phase.
- Human columns are `ID`, `TYPE`, `STATUS`, `CREATED`, and `CANCELABLE`. Error
  and dynamic result payloads remain available in structured output.
- Structured fields are `id`, `type`, `status`, `created_at`, `cancelable`,
  `error`, and `result`.
- Test all statuses and operation types, slash-containing opaque IDs, dynamic
  result and error JSON, pagination, anonymous and authenticated reads, table
  modes, structured output, and API errors.

Phase 2 exit criterion: all four commands are registered on their own branches;
generic API access covers untyped list endpoints; the three typed lists satisfy
the shared paging contract; public and private reads behave correctly;
and the full stack passes cross-platform tests, `go vet`, and lint.

## Phase 3: composed reads

Status: complete. The two-PR stack was merged on 2026-09-04:

```text
main
  release/view
    pr/view
```

These remain read-only but need multiple requests or client-side lookup.

| Order | Branch / PR | Command | API operations | Complexity |
| ---: | --- | --- | --- | --- |
| 3.1 | [`release/view` / #38](https://github.com/dolthub/cli/pull/38) | `dh release view TAG` | `listReleases` | Paginate until an exact tag match; stop early. Replace with `getRelease` if v2 adds it. |
| 3.2 | [`pr/view` / #39](https://github.com/dolthub/cli/pull/39) | `dh pr view NUMBER` | `getPull`, optionally `listPullComments` | Compose metadata and top-level comments while clearly omitting unavailable diff/review data. |

## Phase 4: synchronous create commands

Status: complete. The six-PR stack was merged on 2026-09-04:

These introduce JSON request bodies and interactive/non-interactive input
validation but return their final resource synchronously.

| Order | Branch / PR | Command | API operations | Notes |
| ---: | --- | --- | --- | --- |
| 4.1 | [`db/create` / #41](https://github.com/dolthub/cli/pull/41) | `dh db create` | `getCurrentUser` when owner omitted, `createDatabase` | Small request body; visibility is required. |
| 4.2 | [`branch/create` / #42](https://github.com/dolthub/cli/pull/42) | `dh branch create` | `createBranch` | Mutually exclusive `--from-branch` and `--from-commit` map directly to the v2 union. |
| 4.3 | [`tag/create` / #43](https://github.com/dolthub/cli/pull/43) | `dh tag create` | `createTag` | Builds on branch-create validation; optional message distinguishes annotated and lightweight tags. |
| 4.4 | [`release/create` / #44](https://github.com/dolthub/cli/pull/44) | `dh release create` | `createRelease` | Adds body/body-file handling and optional tag creation. |
| 4.5 | [`pr/comment` / #45](https://github.com/dolthub/cli/pull/45) | `dh pr comment` | `createPullComment` | Simple mutation plus editor/stdin/body-source behavior. |
| 4.6 | [`pr/create` / #46](https://github.com/dolthub/cli/pull/46) | `dh pr create` | `createPull` | Hardest synchronous create due to prompts and same-database versus cross-fork branch references. |

## Phase 5: synchronous PR state changes

Status: complete. The three-PR stack was merged on 2026-09-04:

These share PATCH semantics but remain separate command branches and PRs. Start
with the fixed state transitions, then add the more general editing surface.

| Order | Branch / PR | Command | API operations | Notes |
| ---: | --- | --- | --- | --- |
| 5.1 | [`pr/close` / #48](https://github.com/dolthub/cli/pull/48) | `dh pr close NUMBER` | `updatePull` | Establish PATCH behavior with the fixed payload `state: closed`. |
| 5.2 | [`pr/reopen` / #49](https://github.com/dolthub/cli/pull/49) | `dh pr reopen NUMBER` | `updatePull` | Sends only `state: open` and reuses the typed PATCH client. |
| 5.3 | [`pr/edit` / #50](https://github.com/dolthub/cli/pull/50) | `dh pr edit NUMBER` | `updatePull` | Adds general partial-update validation and field-presence semantics. |

Each remains a distinct PR even though `close` and `reopen` are small. Their
separate branches make the one-command policy explicit and allow independent
review/revert.

## Phase 6: asynchronous operation framework and commands

Status: complete. The four-PR stack was merged on 2026-09-04:

Async behavior is a shared reliability boundary and gets a foundation PR
before any async command.

| Order | Branch / PR | Command or scope | API operations | Notes |
| ---: | --- | --- | --- | --- |
| 6.1 | [`core/operation-waiter` / #52](https://github.com/dolthub/cli/pull/52) | Internal waiter | `getOperation` | Same-origin href validation, bounded exponential backoff with jitter, cancellation, terminal failure rendering, and deterministic fake-clock tests. |
| 6.2 | [`operation/watch` / #53](https://github.com/dolthub/cli/pull/53) | `dh operation watch ID` | `getOperation` | Exposes the waiter directly with `--interval`; returns nonzero for failed operations. |
| 6.3 | [`db/fork` / #54](https://github.com/dolthub/cli/pull/54) | `dh db fork [DB]` | `getCurrentUser`, `createFork`, `getOperation` | Wait by default; `--no-wait` exports the initial OperationRef. |
| 6.4 | [`pr/merge` / #55](https://github.com/dolthub/cli/pull/55) | `dh pr merge NUMBER` | `mergePull`, `getOperation` | One server-defined merge mode; wait by default. |

## Phase 7: SQL

Status: next. Start a new one-PR phase from `main`.

SQL is one leaf command with read and explicitly selected write modes, so both
modes belong in the same `sql` branch and PR. Splitting modes across PRs would
temporarily ship an incomplete command contract and violate the one-command
ownership rule.

| Order | Branch / PR | Command | API operations | Notes |
| ---: | --- | --- | --- | --- |
| 7.1 | `sql` | `dh sql` | `runSqlReadQueryPost`, `runSqlWriteQuery`, `getOperation` | Accept one of positional SQL, `--file`, or piped stdin. Reads render typed columns/rows and interpret query-level status. Writes require `--write`, branch inputs, auth, and async waiting. |

### Phase 7.1: `sql`

This is one branch and one PR. It includes the typed client methods because no
separate foundation is needed for methods used by only this command. The PR
must add and register the `sql` leaf only when both modes are complete.

#### Command and input contract

- Accept at most one positional query. Prefer an explicit positional argument
  or `--file FILE`, which are mutually exclusive; with neither, fall back to
  non-TTY stdin. `--file -` explicitly selects stdin. Reject no source, read
  errors, and whitespace-only SQL. Preserve the selected bytes without
  statement parsing or mutation.
- Resolve `-R/--db` through the shared database resolver. Read mode remains
  usable anonymously for a public database; write mode requires credentials.
- Default to read mode. `--write` is an explicit safety boundary and must never
  be inferred from SQL text.
- Add `--json`, `--jq`, and `--template`. Validate the selected fields against
  the mode-specific resource rather than accepting read fields for writes or
  operation fields for reads.

#### Read mode

- Require `--ref REF`; reject the write-only flags `--branch`,
  `--from-branch`, and `--no-wait`.
- Add `Client.RunSQLRead` and call `runSqlReadQueryPost` with
  `SQLReadRequest { ref, q, limit?, timeout_ms? }`. Always use POST, including
  for short queries; leave the GET variant to `dh api`.
- Treat omitted `--limit` and `--timeout` as absent JSON fields so server
  defaults remain authoritative. Validate an explicit limit as positive.
  Validate timeout as positive, at most 60 seconds, and exactly representable
  in whole milliseconds before converting it to `timeout_ms`.
- Render column names as table headers and rows in server order. Render null as
  `NULL` and use no client-side SQL type coercion. Escape embedded backslashes,
  tabs, carriage returns, and newlines in human output so row boundaries remain
  stable; structured output preserves original strings and nulls. Reject
  inconsistent row widths. Use aligned TTY tables and stable tab-separated
  non-TTY output.
- In human mode, write warnings and a non-success server message to stderr. In
  structured mode, export `columns`, `rows`, `status`, `message`, and
  `warnings` without changing their types. After rendering the response, return
  failure for every status except `success`, including unknown statuses.

#### Write mode

- Require nonempty `--branch` as the write target (`to_branch`). Accept optional
  `--from-branch` as the base and default it to `--branch` for in-place writes.
  Reject read-only `--ref`, `--limit`, and `--timeout`.
- Add `Client.RunSQLWrite` and call `runSqlWriteQuery` with
  `SQLWriteRequest { from_branch, to_branch, q }`. Pass branch names through;
  do not infer, create, or validate them locally beyond nonemptiness.
- Reuse the Phase 6 waiter and terminal status reporter. Wait by default,
  render the terminal operation, and propagate terminal failure. With
  `--no-wait`, render the accepted `OperationRef` and do not poll.
- Human and structured operation output must match `db fork` and `pr merge`.
  Progress always goes to stderr so JSON, jq, templates, and piped output stay
  clean.

#### Required tests and exit criterion

- Constructor and root-registration tests cover help, every flag, positional
  argument count, database resolution, and mode-specific flag conflicts.
- Input tests cover positional SQL, regular files, `--file -`, piped stdin,
  TTY-without-input, conflicting sources, empty input, file errors, multiline
  SQL, and preservation of large body-encoded queries.
- Mocked HTTP tests assert escaped owner/database paths, POST methods, exact
  request JSON, optional-field omission, anonymous public reads, authenticated
  private reads, required write auth, v2 problem errors, and cancellation.
- Read-result tests cover no rows, dynamic column counts, nulls, tabs/newlines,
  binary and temporal strings, duplicate column labels, malformed row widths,
  warnings, all documented statuses, an unknown status, limit boundaries, and
  timeout conversion/boundaries.
- Output tests cover TTY and non-TTY tables plus every read structured field.
  Write tests cover the accepted reference, queued/running/succeeded progress,
  failed operations, `--no-wait`, and every operation structured field.
- Phase 7 is complete when the single PR passes the repository test, vet, lint,
  and whitespace checks; read SQL works against a public database; write SQL
  completes through the shared waiter; and root help advertises the complete
  `sql` leaf.

## Phase 8: multipart import

This is last because it combines filesystem I/O, multipart planning, direct
object-storage requests, checksums, a large API request, progress reporting,
and asynchronous completion.

| Order | Branch / PR | Command | API operations | Notes |
| ---: | --- | --- | --- | --- |
| 8.1 | `db/import` | `dh db import FILE` | `createImportUpload`, pre-signed part PUTs, `createImport`, `getOperation` | Stream parts without loading the whole file; propagate required upload headers; collect ETags; compute aggregate MD5; wait by default; support all documented file types and import modes. |

The import PR should include a fake object-storage server, multipart boundary
and retry tests, checksum fixtures, interrupted-upload behavior, and tests that
credentials are never forwarded to pre-signed storage origins.

## Phase 9: commands blocked on new v2 endpoints

These commands have direct `gh` equivalents but cannot be implemented from the
current public v2 contract. Their `dh` branches must not start until the
corresponding `ld` endpoint is merged, documented in OpenAPI, and deployed to
the development environment.

| Order | External prerequisite in `ld` | Branch / PR in `dh` | Command |
| ---: | --- | --- | --- |
| 9.1 | Add cursor-paginated `GET /api/v2/databases?owner=OWNER` (`listDatabases`) | `db/list` | `dh db list [OWNER]` |
| 9.2 | Add cursor-paginated `GET /api/v2/user/organizations` (`listCurrentUserOrganizations`) | `org/list` | `dh org list` |

The API endpoint changes are separate PRs in `ld`; each CLI command still gets
its own branch and PR in `dh`.

`dh db list` should match the useful subset of `gh repo list [OWNER]`: default
to the authenticated user, accept a user or organization owner, paginate, and
support `--limit`, `--visibility`, and structured output. Only expose filters
implemented by the v2 endpoint.

`dh org list` should match `gh org list`: list organizations for the
authenticated user with `--limit` and structured output. There is no need for
`dh org view` merely for command parity.

## Commands intentionally outside this roadmap

The following remain omitted until public v2 gains the necessary resources:

- Database edit, rename, delete, archive, clone/sync integration, stars,
  collaborators, permissions, default-branch management, and file reads.
- PR diff, checkout, checks, review, ready, update-branch, revert, lock, inline
  comments, and comment editing/deletion.
- Release edit/delete, assets, generated notes, drafts, and prereleases.
- Issues, search, organization administration, Dolt CI configuration/control,
  secrets, variables, projects, labels, and operation cancellation.

When v2 adds one of these capabilities, insert the command into the earliest
phase whose complexity profile fits it. Keep the one-command-per-branch/PR
rule rather than appending multiple newly available commands to one change.

## Summary order

Excluding existing commands and internal foundation PRs, the proposed command
PR order is:

```text
operation/watch
db/fork
pr/merge
sql
db/import
db/list       (blocked on new v2 endpoint)
org/list      (blocked on new v2 endpoint)
```

Existing commands and commands completed in Phases 1–6 stay on `main` and are
not bundled into new command PRs.
