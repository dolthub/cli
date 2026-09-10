# dh

`dh` is the official command-line interface for [DoltHub](https://www.dolthub.com).

The CLI targets Linux, macOS, and Windows and requires Go 1.26 or newer to build
from source.

## Install

Download the archive for your operating system and architecture from
[GitHub Releases](https://github.com/dolthub/cli/releases), verify it against
`checksums.txt`, and extract `dh` (`dh.exe` on Windows) into a directory on your
`PATH`.

The Docker image is `dolthub/cli`, with version tags and `latest`:

```sh
docker run --rm dolthub/cli:latest version
```

See [Docker usage](docker/README.md) for authentication and mounted files.

## Build

```sh
make build
./bin/dh version
```

Run the test suite with `make test` and format Go sources with `make fmt`.

## Authentication

Log in to production DoltHub using your browser:

```sh
./bin/dh auth login
./bin/dh auth status
```

The default host is `www.dolthub.com`, and ordinary builds include the public
DoltHub CLI OAuth client ID. Login uses PKCE without a client secret, stores
your credentials, and refreshes access tokens automatically.

If `DH_HOST` or your saved configuration selects another host, use
`./bin/dh auth login --hostname www.dolthub.com` to log in to production.
`DH_OAUTH_CLIENT_ID` optionally overrides the built-in client ID; development
and custom hosts require their own client ID. Credentials issued to another
OAuth client require that client's override or a fresh login with the default.

You can also authenticate with `DH_TOKEN`. Unset it before using browser login.

## SQL

Run a read query against a branch, tag, or commit:

```sh
dh sql --db OWNER/DATABASE --ref main "select * from table_name limit 10"
```

SQL can also come from a file or pipe. Use `--json columns,rows,status` for
structured results.

Writes are explicit and asynchronous. `--branch` is the target; an optional
`--from-branch` supplies the base when creating or updating a feature branch:

```sh
dh sql --write --db OWNER/DATABASE --branch main \
  "update table_name set value = 1 where id = 42"

dh sql --write --db OWNER/DATABASE --branch feature/update \
  --from-branch main --file update.sql
```

Write commands wait for completion and display job status by default.
Pass `--no-wait` to print the accepted job reference immediately.
Job IDs in tables, progress, and JSON output use the UUID rather than the
fully qualified resource name. The `href` field remains the full polling URL.

## Table imports

Upload a local file and import it into a DoltHub table:

```sh
dh table import people people.csv --db OWNER/DATABASE --branch main \
  --primary-key id --message "Import people"

dh table import people changes.json --db OWNER/DATABASE --branch main --update
```

The command creates a table by default. Use one of `--overwrite`, `--update`,
or `--replace` to import into an existing table. Supported formats are CSV,
PSV, XLSX, and JSON; JSON requires `--update` or `--replace`. The format is
inferred from the extension, or supplied with `--file-type`. Primary keys
can be comma-separated or supplied with repeated `--primary-key` flags.

`--branch` is required because API v2 does not expose the database's default
branch. `--db` can be omitted when a local Dolt remote identifies the database.
Files must be regular, nonempty files of at most 1 GiB; stdin is not supported.
Keep the file unchanged during upload.

Uploads use multipart storage URLs and wait for import completion by default.
`--no-wait` returns the job reference after uploading and submitting the
import. Use `--json id,status,result` for structured completion output, or
`--no-wait --json id,href` for the accepted job reference.

Storage URLs expire after 10 minutes. Failed or expired uploads must be
restarted; this version does not refresh URLs or resume uploads. Ctrl+C stops
local transfers but does not abort the storage session or cancel an already
submitted import. After a submission error, check `dh job list --db
OWNER/DATABASE` before retrying, since the import may already have been accepted.
