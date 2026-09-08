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

Write commands wait for completion and display operation status by default.
Pass `--no-wait` to print the accepted operation reference immediately.
