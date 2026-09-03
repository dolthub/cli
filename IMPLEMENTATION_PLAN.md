# `dh` phased implementation plan

Status: proposed

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
  core/database-resolver
    db/view
      branch/list
        tag/list
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

These are internal capability PRs, not command PRs. They are ordered so each
one is independently testable and leaves the existing command surface working.

| Order | Branch | Scope |
| ---: | --- | --- |
| 0.1 | `core/api-client` | Add generic request construction, POST/PATCH JSON bodies, typed success-envelope decoding, pagination metadata, response limits, safe path escaping, and same-origin URL validation. Preserve `CurrentUser`. |
| 0.2 | `core/api-models` | Add v2 DTOs and request types for Database, Branch, Tag, Release, Pull, PullComment, QueryResult, Operation, ImportUpload, and their request bodies. Keep JSON field names aligned with OpenAPI. |
| 0.3 | `core/database-resolver` | Resolve `[HOST/]OWNER/DB` from `-R/--repo`, `DH_REPO`, config, and recognized local Dolt remotes. Keep resolution lazy and injectable. Do not add a `db` command yet. |
| 0.4 | `core/output` | Add table rendering and `gh`-style `--json`, `--jq`, and `--template` support with explicit per-resource field lists. |
| 0.5 | `core/pagination` | Add a cursor pager which treats page tokens as opaque, stops at `--limit`, handles cancellation, and detects a repeated token. |

Phase 0 exit criterion: a command can resolve a database, call any synchronous
v2 JSON operation, paginate a list, and render human or structured output.

## Phase 1: local and single-request read commands

These have no mutation semantics and require either no HTTP request or one
simple GET. They establish command conventions at low risk.

| Order | Branch / PR | Command | API operations | Why here |
| ---: | --- | --- | --- | --- |
| 1.1 | `completion` | `dh completion` | None | Cobra generates completions locally; smallest new command. |
| 1.2 | `config/list` | `dh config list` | None | Exercises stable non-secret tabular output without API behavior. |
| 1.3 | `db/view` | `dh db view [DB]` | `getDatabase` | Simplest database-scoped typed read and first use of resolver/output foundations. |
| 1.4 | `browse` | `dh browse` | None | Reuses database resolution and existing browser abstraction; URL-only behavior. |
| 1.5 | `operation/view` | `dh operation view ID` | `getOperation` | One authenticated GET and important validation for opaque/slash-containing operation IDs. |

`db/view` creates and registers the `db` parent group. `operation/view` creates
and registers the `operation` parent group.

## Phase 2: paginated read commands

Start a new stack after Phase 1 merges. Each command exercises the same paging
and output contract with a different resource shape.

| Order | Branch / PR | Command | API operations | Notes |
| ---: | --- | --- | --- | --- |
| 2.1 | `branch/list` | `dh branch list` | `listBranches` | First cursor-paginated command; creates the `branch` group. |
| 2.2 | `tag/list` | `dh tag list` | `listTags` | Same paging shape with tag-specific fields; creates the `tag` group. |
| 2.3 | `db/forks` | `dh db forks` | `listForks` | Non-paginated today; immediate children only. |
| 2.4 | `release/list` | `dh release list` | `listReleases` | Creates the `release` group and establishes release JSON fields. |
| 2.5 | `pr/list` | `dh pr list` | `listPulls` | Creates the `pr` group; `--state` filtering is client-side because v2 has no filter parameter. |
| 2.6 | `operation/list` | `dh operation list` | `listOperations` | Auth-optional repository-scoped operation history. |

Phase 2 exit criterion: pagination and structured output conventions have been
validated against every principal list envelope currently exposed by v2.

## Phase 3: composed reads

These remain read-only but need multiple requests, client-side lookup, input
encoding, or generalized HTTP behavior.

| Order | Branch / PR | Command | API operations | Complexity |
| ---: | --- | --- | --- | --- |
| 3.1 | `release/view` | `dh release view TAG` | `listReleases` | Paginate until an exact tag match; stop early. Replace with `getRelease` if v2 adds it. |
| 3.2 | `pr/view` | `dh pr view NUMBER` | `getPull`, optionally `listPullComments` | Compose metadata and top-level comments while clearly omitting unavailable diff/review data. |

## Phase 4: synchronous create commands

These introduce JSON request bodies and interactive/non-interactive input
validation but return their final resource synchronously.

| Order | Branch / PR | Command | API operations | Notes |
| ---: | --- | --- | --- | --- |
| 4.1 | `db/create` | `dh db create` | `getCurrentUser` when owner omitted, `createDatabase` | Small request body; visibility is required. |
| 4.2 | `branch/create` | `dh branch create` | `createBranch` | Mutually exclusive `--from-branch` and `--from-commit` map directly to the v2 union. |
| 4.3 | `tag/create` | `dh tag create` | `createTag` | Builds on branch-create validation; optional message distinguishes annotated and lightweight tags. |
| 4.4 | `release/create` | `dh release create` | `createRelease` | Adds body/body-file handling and optional tag creation. |
| 4.5 | `pr/comment` | `dh pr comment` | `createPullComment` | Simple mutation plus editor/stdin/body-source behavior. |
| 4.6 | `pr/create` | `dh pr create` | `createPull` | Hardest synchronous create due to prompts and same-database versus cross-fork branch references. |

## Phase 5: synchronous PR state changes

These share PATCH semantics but remain separate command branches and PRs. Start
with the fixed state transitions, then add the more general editing surface.

| Order | Branch / PR | Command | API operations | Notes |
| ---: | --- | --- | --- | --- |
| 5.1 | `pr/close` | `dh pr close NUMBER` | `updatePull` | Establish PATCH behavior with the fixed payload `state: closed`. |
| 5.2 | `pr/reopen` | `dh pr reopen NUMBER` | `updatePull` | Sends only `state: open` and reuses the typed PATCH client. |
| 5.3 | `pr/edit` | `dh pr edit NUMBER` | `updatePull` | Adds general partial-update validation and field-presence semantics. |

Each remains a distinct PR even though `close` and `reopen` are small. Their
separate branches make the one-command policy explicit and allow independent
review/revert.

## Phase 6: generic API access

This command is intentionally later than typed reads and synchronous mutations.
A safe general-purpose HTTP command has a larger input and security surface
than a narrowly typed operation.

| Order | Branch / PR | Command | API operations | Complexity |
| ---: | --- | --- | --- | --- |
| 6.1 | `api` | `dh api ENDPOINT` | Generic | Method inference, typed fields, raw input, headers, pagination, same-origin enforcement, `--include`, `--silent`, `--jq`, and `--template`. |

## Phase 7: asynchronous operation framework and commands

Async behavior is a shared reliability boundary and gets a foundation PR
before any async command.

| Order | Branch / PR | Command or scope | API operations | Notes |
| ---: | --- | --- | --- | --- |
| 7.1 | `core/operation-waiter` | Internal waiter | `getOperation` | Same-origin href validation, bounded exponential backoff with jitter, cancellation, terminal failure rendering, and deterministic fake-clock tests. |
| 7.2 | `operation/watch` | `dh operation watch ID` | `getOperation` | Exposes the waiter directly with `--interval`; returns nonzero for failed operations. |
| 7.3 | `db/fork` | `dh db fork [DB]` | `getCurrentUser`, `createFork`, `getOperation` | Wait by default; `--no-wait` exports the initial OperationRef. |
| 7.4 | `pr/merge` | `dh pr merge NUMBER` | `mergePull`, `getOperation` | One server-defined merge mode; wait by default. |

## Phase 8: SQL

SQL is one leaf command with read and explicitly selected write modes, so both
modes belong in the same `sql` branch and PR. Splitting modes across PRs would
temporarily ship an incomplete command contract and violate the one-command
ownership rule.

| Order | Branch / PR | Command | API operations | Notes |
| ---: | --- | --- | --- | --- |
| 8.1 | `sql` | `dh sql` | `runSqlReadQueryPost`, `runSqlWriteQuery`, `getOperation` | Accept one of positional SQL, `--file`, or piped stdin. Reads render typed columns/rows and interpret query-level status. Writes require `--write`, branch inputs, auth, and async waiting. |

This phase needs careful tests for dynamic JSON values, nulls, binary/temporal
representations, terminal tables, warnings, row limits, timeouts, SQL-level
HTTP-200 failures, large body-encoded queries, and async writes.

## Phase 9: multipart import

This is last because it combines filesystem I/O, multipart planning, direct
object-storage requests, checksums, a large API request, progress reporting,
and asynchronous completion.

| Order | Branch / PR | Command | API operations | Notes |
| ---: | --- | --- | --- | --- |
| 9.1 | `db/import` | `dh db import FILE` | `createImportUpload`, pre-signed part PUTs, `createImport`, `getOperation` | Stream parts without loading the whole file; propagate required upload headers; collect ETags; compute aggregate MD5; wait by default; support all documented file types and import modes. |

The import PR should include a fake object-storage server, multipart boundary
and retry tests, checksum fixtures, interrupted-upload behavior, and tests that
credentials are never forwarded to pre-signed storage origins.

## Phase 10: commands blocked on new v2 endpoints

These commands have direct `gh` equivalents but cannot be implemented from the
current public v2 contract. Their `dh` branches must not start until the
corresponding `ld` endpoint is merged, documented in OpenAPI, and deployed to
the development environment.

| Order | External prerequisite in `ld` | Branch / PR in `dh` | Command |
| ---: | --- | --- | --- |
| 10.1 | Add cursor-paginated `GET /api/v2/databases?owner=OWNER` (`listDatabases`) | `db/list` | `dh db list [OWNER]` |
| 10.2 | Add cursor-paginated `GET /api/v2/user/organizations` (`listCurrentUserOrganizations`) | `org/list` | `dh org list` |

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
completion
config/list
db/view
browse
operation/view
branch/list
tag/list
db/forks
release/list
pr/list
operation/list
release/view
pr/view
db/create
branch/create
tag/create
release/create
pr/comment
pr/create
pr/close
pr/reopen
pr/edit
api
operation/watch
db/fork
pr/merge
sql
db/import
db/list       (blocked on new v2 endpoint)
org/list      (blocked on new v2 endpoint)
```

Existing commands (`auth login`, `auth logout`, `auth status`, `config get`,
`config set`, and `version`) stay on `main` and are not bundled into new command
PRs.
