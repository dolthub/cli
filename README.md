# dh

`dh` is the official command-line interface for [DoltHub](https://www.dolthub.com).

The CLI targets Linux, macOS, and Windows and requires Go 1.26 or newer to build
from source.

## Build

```sh
make build
./bin/dh version
```

Run the test suite with `make test` and format Go sources with `make fmt`.

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

