# DoltHub CLI

`dolthub/cli` packages `dh`, the official command-line interface for DoltHub.
Images support Linux amd64 and arm64. Use a version tag such as
`dolthub/cli:0.1.0` for a fixed release, or `dolthub/cli:latest` for the newest
completed release.

```sh
docker run --rm dolthub/cli:latest --help
docker run --rm dolthub/cli:latest version
docker run --rm dolthub/cli:latest db view coffeegoddd/bug_repro
```

## Authentication

For private reads and writes, set `DH_TOKEN` in your environment and forward it
to the container. Public reads can run anonymously.

```sh
docker run --rm -e DH_TOKEN dolthub/cli:latest auth status
```

Use the native CLI for browser login. The container has no browser or system
credential service, and its loopback callback is inside the container.
`DH_HOST` selects a custom host when forwarded with `-e DH_HOST`.

## SQL and files

Pass `--db OWNER/DATABASE` explicitly. The image does not include the Dolt
binary for local remote discovery. Add `-i` when piping SQL to stdin:

```sh
printf 'select 1;\n' | docker run --rm -i -e DH_TOKEN dolthub/cli:latest \
  sql --db OWNER/DATABASE --branch main
```

Mount files read-only for `--file` input:

```sh
docker run --rm -e DH_TOKEN \
  --mount "type=bind,src=$PWD,dst=/work,readonly" \
  dolthub/cli:latest sql --db OWNER/DATABASE --branch main --file /work/query.sql
```

The image runs as UID/GID 1001 with a writable home at `/home/dh`. Mounted files
must be readable by that user. The image entrypoint is `dh`; arguments are CLI
commands, and the default command prints help.

Source, native downloads, and issue reporting: <https://github.com/dolthub/cli>.
