# `dh` command and API plan

Status: active — Phases 0 and 1 implemented; Phase 2 specified

This document defines the intended command surface for `dh`. It is based on:

- DoltHub API v2's OpenAPI contract at `ld/web/packages/dolthub/openapi/v2.yaml`
  (`ld` commit `3654e2df4bd7bae81477fd1f05080efb3a2a5b36`).
- The API v2 Next.js handlers and service adapters under
  `ld/web/packages/dolthub/{pages,lib}/api/v2`.
- The backing DoltHub Go services under `ld/go`.
- GitHub CLI's command organization and behavior at `cli` commit
  `b9f99e85254629a7771a43bfa73b423d3b3e3836`.

The OpenAPI document is the source of truth. This plan deliberately does not
promise commands which could only be implemented using DoltHub's private or
legacy APIs.

## Design rules

1. Match `gh` command names, argument order, common flags, output behavior, and
   interactive behavior where DoltHub has the same concept.
2. Expose every useful v2 capability through either a typed command or `dh api`.
3. Add Dolt-native top-level groups only for important concepts without a good
   `gh` home: `branch`, `tag`, `sql`, and `operation`.
4. Use `db` as the top-level command group for DoltHub database repositories.
   This intentionally differs from `gh repo`: "database" is the primary
   DoltHub resource and avoids confusing remote DoltHub operations with local
   Dolt repository operations. Help text may say "database repository" where
   ambiguity matters.
5. Resolve a database in this order: explicit `--repo HOST/OWNER/REPO` or
   `--repo OWNER/REPO`, `DH_REPO`, a recognized Dolt remote in the current Dolt
   repository, then an interactive prompt when appropriate. Scripts fail with
   an actionable error instead of prompting.
6. Public read endpoints work without login. Private reads and all mutations
   require `DH_TOKEN` or stored credentials.
7. List commands follow `meta.next_page_token` until `--limit` results have
   been collected. The token is opaque and must never be parsed.
8. Commands which receive an `OperationRef` wait for completion by default in
   both interactive and non-interactive use. `--no-wait` prints the operation
   reference and exits after acceptance. `--json` never changes this policy;
   callers use `--no-wait` when they want the initial operation reference.
9. Resource-producing commands support `--json`, `--jq`, and `--template` with
   the same meanings as `gh`. Human-readable lists use tables on a terminal and
   stable tab-separated output otherwise.
10. `--web` opens the corresponding DoltHub page and performs no API request.
11. API operation IDs are used as the Go client's method names. The client can
    use shorter Go names where already established, but documentation and tests
    should retain the OpenAPI operation ID for traceability.

## Planned command tree

```text
dh
  api ENDPOINT
  auth
    login
    logout
    status
  branch
    create NAME
    list
  browse [NUMBER]
  completion {bash|fish|powershell|zsh}
  config
    get KEY
    list
    set KEY VALUE
  operation
    list
    view ID
    watch ID
  pr
    close NUMBER
    comment NUMBER
    create
    edit NUMBER
    list
    merge NUMBER
    reopen NUMBER
    view [NUMBER]
  release
    create TAG
    list
    view TAG
  db
    create [OWNER/]NAME
    fork [DATABASE]
    forks [DATABASE]
    import FILE
    list [OWNER]
    view [DATABASE]
  org
    list
  sql [QUERY]
  tag
    create NAME
    list
  version
```

`auth login`, `auth logout`, `auth status`, `browse`, `completion`,
`config get`, `config list`, `config set`, `db view`, `operation view`, and
`version` are implemented. The rest are planned.

## Global conventions

### Common flags

Repository-scoped commands accept:

```text
-R, --repo [HOST/]OWNER/REPO
```

List and resource commands accept, where applicable:

```text
--limit N
--json FIELDS
--jq EXPRESSION
--template STRING
-w, --web
```

`--hostname` remains an authentication/configuration flag. `--repo` carries a
host when a one-off command targets a non-default host.

### Async operations

`createFork`, `runSqlWriteQuery`, `createImport`, and `mergePull` return HTTP
202 with an `OperationRef`. The shared waiter must:

1. Follow `OperationRef.href` instead of constructing a path from `id` because
   operation IDs may contain slashes.
   Validate that the href uses the configured DoltHub origin before forwarding
   credentials.
2. Poll `getOperation` with bounded exponential backoff and jitter.
3. Stop on `succeeded` or `failed`, or on context cancellation.
4. Render `Operation.error` and return failure when the terminal state is
   `failed`.
5. Support `--no-wait`; later, a global `--timeout` may bound waiting.

Operation status is one of `queued`, `running`, `succeeded`, or `failed`.
Operation type is one of `import`, `merge`, `sql_write`, `fork`, or `dolt_ci`.

### API errors and query errors

HTTP failures use the v2 RFC 9457 `Problem` response and should retain status,
error code, request ID, method, and path. SQL-level failures are different:
`runSqlReadQuery` returns HTTP 200 with `QueryResult.status` set to `error`,
`timeout`, `row_limit`, or `not_workspace`. `dh sql` must treat every status
other than `success` as a command failure after printing the server message.

## Command specifications

### `dh api`

Low-level access to API v2, modeled after `gh api`.

```text
dh api ENDPOINT [--method METHOD] [-f key=value] [-F key=value]
                [--input FILE] [--paginate] [--slurp]
                [--include] [--silent] [--hostname HOST]
                [--jq EXPRESSION] [--template STRING]
```

- `ENDPOINT` is relative to `/api/v2/` by default. A full `/api/v2/...` path is
  also accepted.
- Default method is GET, or POST when body fields are supplied.
- `--paginate` follows `meta.next_page_token` by setting `page_token` on the
  next request. It is valid only for v2 envelope responses.
- This uses the generic authenticated HTTP client rather than a typed method.
- Arbitrary absolute URLs are rejected so credentials cannot be forwarded to a
  different origin.

This command is the escape hatch for API features before a typed command ships.

### `dh auth`

| Command | API use | Notes |
| --- | --- | --- |
| `auth login` | OAuth authorize/token endpoints, then `getCurrentUser` | Already implemented. Browser OAuth authorization-code flow with PKCE. |
| `auth logout` | None | Already implemented. Deletes the selected local credential. There is no v2 token-revocation operation. |
| `auth status` | `getCurrentUser` | Already implemented. Validates the effective token without printing it. |

Do not copy `gh auth refresh`, `setup-git`, `git-credential`, `switch`, or
`token` until there is a concrete DoltHub need. OAuth refresh is automatic;
Dolt remotes do not currently need `dh` as a Git credential helper; and a
command must not print stored secrets by default.

### `dh config`

| Command | API use | Notes |
| --- | --- | --- |
| `config get KEY` | None | Already implemented. |
| `config set KEY VALUE` | None | Already implemented. |
| `config list` | None | Print all supported effective values and their source, redacting secrets. |

Initial keys are `host` and `repo`. Environment overrides remain `DH_HOST` and
`DH_REPO`.

`config list` prints `KEY`, `VALUE`, and `SOURCE`, where source is
`environment`, `config`, `default`, or `unset`. Terminal output has headers and
aligned columns; piped output is stable tab-separated data without headers. An
invalid `DH_REPO` is an error and does not fall back to the persisted value.

### `dh completion`

```text
dh completion {bash|fish|powershell|zsh}
```

Generate a completion script from the registered Cobra command tree and write
it to stdout. Completion generation is entirely local and must not load
configuration, credentials, or an HTTP client.

### `dh browse`

```text
dh browse [NUMBER] [-R REPOSITORY]
dh browse --pull NUMBER [-R REPOSITORY]
dh browse --branch NAME [-R REPOSITORY]
```

No API call is required. Build a URL from the resolved host and repository and
open it with the system browser. With no selector, open
`/repositories/{owner}/{database}`. A numeric positional argument or `--pull`
opens `/repositories/{owner}/{database}/pulls/{number}`; `--branch` opens
`/repositories/{owner}/{database}/data/{branch}`. Selectors are mutually
exclusive, and every dynamic path segment is escaped. Arbitrary paths and
ambiguous ref guessing are intentionally unsupported.

### `dh db create`

```text
dh db create [OWNER/]NAME [--description TEXT] [--public | --private]
```

Calls:

```text
createDatabase
POST /api/v2/databases
CreateDatabaseRequest { owner, name, description?, visibility }
```

Behavior:

- Use `getCurrentUser` as the owner when `[OWNER/]` is omitted. Prompt for name,
  visibility, and optional description when omitted on a terminal.
- In non-interactive use, require the name and one visibility flag. This is
  stricter than `gh repo create` because v2 requires `visibility`.
- Do not offer `gh` flags such as `--add-readme`, `--gitignore`, `--license`,
  `--source`, `--push`, or `--team`; v2 cannot implement them.
- Print `[HOST/]OWNER/NAME` and its web URL after creation.

### `dh db list`

```text
dh db list [OWNER] [--limit N] [--visibility {public|private}]
```

Blocked on a new cursor-paginated v2 `listDatabases` operation, proposed as
`GET /api/v2/databases?owner=OWNER&page_token=...`. With no OWNER, use the
authenticated user. OWNER may identify a user or organization. This is the
counterpart to `gh repo list [OWNER]`; only filters supported by the eventual
v2 contract should be exposed.

### `dh db view`

```text
dh db view [DATABASE] [--forks] [--web]
```

Calls:

```text
getDatabase
GET /api/v2/databases/{owner}/{database}

listForks                    only with --forks
GET /api/v2/databases/{owner}/{database}/forks
```

The normal view shows owner/name, visibility, description, size, stars, last
write time, parent, network root, and fork-network count. Unlike `gh repo view`,
it cannot show a README, license, topics, or default branch because those are
not present in the v2 `Database` resource.

`--forks` adds immediate child forks. `listForks` is not paginated today.
`--web` makes no API request and is incompatible with `--forks` and structured
output. Structured fields use API v2 snake_case names; `forks` is exposed only
when requested.

### `dh db fork`

```text
dh db fork [DATABASE] [--org OWNER] [--no-wait]
```

Calls:

```text
createFork
POST /api/v2/databases/{owner}/{database}/forks
CreateForkRequest { owner }

getOperation                  unless --no-wait
GET OperationRef.href
```

`--org` matches the familiar `gh repo fork` spelling and selects the target
user or organization. Without it, use the authenticated username from
`getCurrentUser`. API v2 always preserves the source database name, so there is
no `--fork-name`. `--clone` and `--remote` are deferred until local Dolt
integration is designed.

### `dh db forks`

```text
dh db forks [DATABASE] [-R REPOSITORY]
```

Calls `listForks`. This is a DoltHub-specific addition because fork-network
topology is a first-class database concept and v2 exposes it directly. It lists
immediate children only; `Database.fork_network_count` is the transitive count.
The endpoint returns the complete bounded list in one response, so this command
does not expose `--limit`. Structured fields are `owner` and `name`.

### `dh db import`

```text
dh db import FILE --table TABLE [--branch BRANCH]
    [--create | --overwrite | --update | --replace]
    [--primary-key COLUMN]... [--map SOURCE=DEST]...
    [--commit-message TEXT] [--pr-branch NAME] [--no-wait]
```

This is one high-level command over a multi-step protocol:

1. Inspect the file and choose a safe multipart chunk count.
2. Call `createImportUpload` with file size, part count, and file type.
3. PUT each chunk to the returned pre-signed URL using every returned header;
   retain each ETag and compute the required aggregate base64 MD5.
4. Call `createImport` with the upload token, contents key, completed parts,
   table/import settings, and hashes.
5. Poll `getOperation` unless `--no-wait`.

API methods:

```text
createImportUpload
POST /api/v2/databases/{owner}/{database}/imports/uploads
CreateImportUploadRequest { content_length, num_parts, file_type }

createImport
POST /api/v2/databases/{owner}/{database}/imports
CreateImportRequest { branch_name, table_name, file_name, file_size,
  file_type, import_operation, token, contents_key, completed_parts,
  file_parts_md5, primary_keys, commit_message?,
  pull_request_branch_name?, column_map? }

getOperation
GET OperationRef.href
```

Supported file types are `csv`, `psv`, `xlsx`, `json`, `sql`, and `yaml`.
Import modes are `create`, `overwrite`, `update`, and `replace`. Default branch
selection should come from explicit `--branch`; v2 does not expose a database's
default branch, so no implicit `main` should be baked into the client.

### `dh branch`

```text
dh branch list [-R REPOSITORY] [--limit N]
dh branch create NAME (--from-branch NAME | --from-commit SHA)
```

| Command | API operation | Request |
| --- | --- | --- |
| `branch list` | `listBranches` | `GET .../branches?page_token=...` |
| `branch create` | `createBranch` | `POST .../branches` with `{ name, from: { branch } }` or `{ name, from: { commit } }` |

`branch list` defaults to `--limit 30`, follows cursor pagination, and shows
name, head commit SHA, and last update time. Structured fields are `name`,
`head_commit_sha`, and `last_updated_at`. The create source flags are mutually
exclusive and exactly mirror the API's discriminated union. There is no view,
rename, or delete command because v2 has no corresponding operation.

### `dh tag`

```text
dh tag list [-R REPOSITORY] [--limit N]
dh tag create NAME (--from-branch NAME | --from-commit SHA) [--message TEXT]
```

| Command | API operation | Request |
| --- | --- | --- |
| `tag list` | `listTags` | `GET .../tags?page_token=...` |
| `tag create` | `createTag` | `POST .../tags` with `{ name, from, message? }` |

Omitting `--message` creates a lightweight tag; supplying it creates an
annotated tag. `tag list` defaults to `--limit 30`; its structured fields are
`name`, `commit_sha`, `message`, and `tagged_at`. There is no view or delete
command in v2.

### `dh sql`

```text
dh sql [QUERY] --ref REF [--limit N] [--timeout DURATION]
dh sql --file FILE --ref REF [--limit N] [--timeout DURATION]
dh sql --write [QUERY] --from-branch NAME --to-branch NAME [--no-wait]
```

Read mode calls `runSqlReadQueryPost`:

```text
POST /api/v2/databases/{owner}/{database}/sql
SqlReadRequest { ref, q, limit?, timeout_ms? }
```

The CLI should prefer the body-encoded POST operation for all reads, avoiding
URL-length limits while retaining identical public-read semantics. The GET
`runSqlReadQuery` operation remains available through `dh api`.

Write mode calls `runSqlWriteQuery`, then `getOperation` unless `--no-wait`:

```text
POST /api/v2/databases/{owner}/{database}/sql-writes
SqlWriteRequest { from_branch, to_branch, q }
```

`--write` must be explicit. The client must not guess whether arbitrary SQL is
read-only. SQL may be supplied as one positional argument, by `--file`, or from
stdin when stdin is not a terminal. These sources are mutually exclusive.

Read output is a table by default. `--json` exports the complete `QueryResult`,
including column metadata, rows, status, message, and warnings. The server
defaults are 1,000 rows and 30 seconds; its documented timeout cap is 60
seconds.

### `dh pr list`

```text
dh pr list [-R REPOSITORY]
           [--state {open|closed|merged|all}] [--limit N]
```

Calls `listPulls` and follows cursor pagination. `--limit` defaults to 30 and
`--state` defaults to `open`. API v2 provides no server-side filters, so
`--state` is applied client-side before the limit is counted; the command
continues fetching pages until it has enough matching rows or reaches the end.
The structured fields are `pull_number`, `title`, `description`, `state`,
`created_at`, and `creator`. Do not copy `gh`
filters for author, assignee, labels, reviews, checks, base, head, search, or
draft state because v2 does not expose those concepts.

### `dh pr create`

```text
dh pr create --title TEXT [--body TEXT | --body-file FILE]
             --head [OWNER/REPO:]BRANCH --base BRANCH
```

Calls:

```text
createPull
POST /api/v2/databases/{owner}/{database}/pulls
CreatePullRequest { title, description?, from_branch, to_branch }
```

`--head` may identify a branch in a fork. `--base` is always in the target
repository selected by `--repo`; v2 requires `to_branch.database` to match the
URL repository. Interactive mode prompts for omitted fields. There are no
reviewer, assignee, label, project, draft, or maintainer-edit flags.

### `dh pr view`

```text
dh pr view [NUMBER] [--comments] [--web]
```

Calls `getPull`, plus `listPullComments` with `--comments`. The API exposes only
top-level comments, not inline diff conversations. If NUMBER is omitted, the
CLI may infer a pull only when local Dolt branch/remote integration can do so
unambiguously; until then NUMBER is required outside interactive selection.

The view cannot show a diff, checks, reviews, mergeability, or commits because
v2 does not return them.

### `dh pr edit`, `close`, and `reopen`

```text
dh pr edit NUMBER [--title TEXT] [--body TEXT | --body-file FILE]
dh pr close NUMBER
dh pr reopen NUMBER
```

All call `updatePull`:

```text
PATCH /api/v2/databases/{owner}/{database}/pulls/{pull_number}
UpdatePullRequest { title?, description?, state?: open|closed }
```

At least one field is required. `close` sends `state: closed`; `reopen` sends
`state: open`. A pull reaches `merged` only through `mergePull`.

### `dh pr comment`

```text
dh pr comment NUMBER [--body TEXT | --body-file FILE]
```

Calls `createPullComment` with `{ body }`. With no body source on a terminal,
open the configured editor or prompt. Editing or deleting a comment is not
supported by v2.

### `dh pr merge`

```text
dh pr merge NUMBER [--no-wait]
```

Calls `mergePull`, then `getOperation` unless `--no-wait`. V2 exposes one
server-defined merge behavior, so do not offer GitHub-specific merge strategy,
commit-title, commit-message, auto-merge, branch deletion, or admin flags.

### `dh release list`

```text
dh release list [-R REPOSITORY] [--limit N]
```

Calls `listReleases`, defaults to `--limit 30`, and follows cursor pagination.
Structured fields are `tag`, `title`, `commit_sha`, `description`,
`created_at`, and `updated_at`. V2 has no draft,
prerelease, author, asset, or latest-release concepts.

### `dh release create`

```text
dh release create TAG --title TEXT --target COMMIT
                  [--notes TEXT | --notes-file FILE]
                  [--create-tag]
```

Calls:

```text
createRelease
POST /api/v2/databases/{owner}/{database}/releases
CreateReleaseRequest { tag, title, commit_sha, description?,
  create_tag_if_not_exists? }
```

`--target` is required because v2 requires an exact commit SHA. `--create-tag`
maps to `create_tag_if_not_exists`. Assets, generated notes, discussion links,
drafts, prereleases, and latest-release designation are unsupported.

### `dh release view`

```text
dh release view TAG [--web]
```

API v2 has no get-release operation. The command calls `listReleases`, follows
pagination until the exact tag is found, and stops early on a match. This is a
client-side compatibility command and may be less efficient than `gh release
view`. If the server later adds `getRelease`, switch without changing the CLI.

### `dh operation`

```text
dh operation list [-R REPOSITORY] [--limit N]
dh operation view ID
dh operation watch ID [--interval DURATION]
```

| Command | API operation | Notes |
| --- | --- | --- |
| `operation list` | `listOperations` | Repository-scoped, cursor-paginated. |
| `operation view` | `getOperation` | Operation ID is accepted as opaque input and encoded safely. |
| `operation watch` | `getOperation` | Poll until terminal state; return failure for `failed`. |

`operation list` defaults to `--limit 30`, is authentication-optional like the
other repository-scoped public reads, and includes Dolt CI jobs even though v2
currently has no command to create or configure them. Structured fields match
`operation view`: `id`, `type`, `status`, `created_at`, `cancelable`, `error`,
and `result`.

`operation view` requires authentication and treats the ID as one opaque path
segment, including when it contains slashes. Its structured fields are `id`,
`type`, `status`, `created_at`, `cancelable`, `error`, and `result`. Viewing an
already-failed operation displays its recorded error but exits successfully;
`operation watch` owns terminal-operation failure semantics.

### `dh org list`

```text
dh org list [--limit N]
```

Blocked on a new cursor-paginated v2 `listCurrentUserOrganizations` operation,
proposed as `GET /api/v2/user/organizations?page_token=...`. It lists
organizations the authenticated user belongs to and corresponds to `gh org
list`. Database listing for an organization remains `dh db list OWNER`; no
separate `dh org list-dbs` command is needed.

## Typed API client methods

The initial `internal/dolthub.Client` should grow to the following surface. The
names in parentheses are the OpenAPI operation IDs when a shorter Go name is
used.

```go
CurrentUser(ctx) (User, error) // getCurrentUser

CreateDatabase(ctx, CreateDatabaseRequest) (Database, error)
GetDatabase(ctx, owner, database string) (Database, error)

ListBranches(ctx, owner, database, pageToken string) (Page[Branch], error)
CreateBranch(ctx, owner, database string, req CreateBranchRequest) (Branch, error)
ListTags(ctx, owner, database, pageToken string) (Page[Tag], error)
CreateTag(ctx, owner, database string, req CreateTagRequest) (Tag, error)
ListForks(ctx, owner, database string) ([]Database, error)
CreateFork(ctx, owner, database string, req CreateForkRequest) (OperationRef, error)

ListReleases(ctx, owner, database, pageToken string) (Page[Release], error)
CreateRelease(ctx, owner, database string, req CreateReleaseRequest) (Release, error)

RunSQLRead(ctx, owner, database string, req SQLReadRequest) (QueryResult, error)
RunSQLWrite(ctx, owner, database string, req SQLWriteRequest) (OperationRef, error)

ListPulls(ctx, owner, database, pageToken string) (Page[PullSummary], error)
CreatePull(ctx, owner, database string, req CreatePullRequest) (Pull, error)
GetPull(ctx, owner, database string, number int) (Pull, error)
UpdatePull(ctx, owner, database string, number int, req UpdatePullRequest) (Pull, error)
ListPullComments(ctx, owner, database string, number int) ([]PullComment, error)
CreatePullComment(ctx, owner, database string, number int, req CreatePullCommentRequest) (PullComment, error)
MergePull(ctx, owner, database string, number int) (OperationRef, error)

CreateImportUpload(ctx, owner, database string, req CreateImportUploadRequest) (ImportUpload, error)
CreateImport(ctx, owner, database string, req CreateImportRequest) (OperationRef, error)

ListOperations(ctx, owner, database, pageToken string) (Page[Operation], error)
GetOperation(ctx, href string) (Operation, error)
```

The shared transport layer needs generic GET/POST/PATCH helpers, JSON request
encoding, v2 envelope decoding, pagination metadata, response-size limits, and
status-aware decoding. Path segments must be escaped individually. In
particular, operation IDs must not be inserted into URL paths without escaping;
following the server-provided `href` is preferred.

## `gh` parity matrix and API gaps

| `gh` area | `dh` decision | Reason |
| --- | --- | --- |
| `api` | Implement | Generic v2 HTTP access is useful and fully supportable. |
| `auth` | Partial parity | Login/logout/status exist; Git-specific credential commands do not apply. |
| `browse` | Implement | Pure URL construction; no API gap. |
| `completion` | Implement | Cobra can generate it locally. |
| `config` | Core parity | Get/set/list are local concerns. |
| `repo` | Use `dh db`: create/view/fork plus DoltHub list/import/forks | Current v2 supports create, get, fork, fork listing, and import; list is blocked on a proposed endpoint. |
| `pr` | Create/list/view/edit/close/reopen/comment/merge | Direct v2 operations exist. |
| `release` | Create/list/view-by-listing | V2 supports create/list; view can scan by tag. |
| `status` | Defer | No user dashboard, notification, or cross-repository PR endpoint. |
| `search` | Omit | No v2 search endpoints. |
| `org` | Plan `org list` | Blocked on a proposed current-user organization-membership endpoint. |
| `issue` | Omit | DoltHub issues are not exposed by v2. |
| `discussion` | Omit | No v2 discussion resources. |
| `alias`, `extension` | Defer | Local extensibility can be added later and is not needed for API coverage. |
| Actions/workflow/run/cache | Omit | Dolt CI only appears as an operation type; v2 exposes no CI configuration or run controls. |
| project/label/ruleset | Omit | No v2 resources. |
| secret/variable | Omit | No v2 resources. |
| gist/codespace/keys/attestation | Omit | GitHub-specific concepts. |

Specific familiar `gh` commands which cannot currently be implemented include:

- `repo list`, `clone`, `edit`, `rename`, `delete`, `archive`, `sync`, file
  reads, deploy keys, licenses, and autolinks.
- `pr diff`, `checkout`, `checks`, `review`, `ready`, `update-branch`, `revert`,
  lock/unlock, inline comments, and comment editing/deletion.
- `release edit`, `delete`, upload/download/delete assets, and verification.
- Branch or tag deletion/renaming and commit/history/log commands.
- Database stars, collaborators, permissions, and default-branch management.
- Database enumeration and current-user organization memberships until their
  proposed v2 list operations are available.
- Canceling an operation even though `Operation.cancelable` exists. V2 has no
  cancel endpoint.

These omissions should remain visible in planning. A missing typed command is a
CLI backlog item; a missing v2 operation is an API backlog item and should not
be worked around with private RPCs.

## Delivery plan

See [IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md) for the phased delivery
order, branch names, pull request boundaries, dependencies, and definition of
done. Every executable leaf command is implemented on its own branch and in its
own pull request. The command tree only advertises a command once its
implementation is complete.
