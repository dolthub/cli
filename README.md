# dh

`dh` is the official command-line interface for DoltHub. It is under active
development and does not yet provide user-facing DoltHub workflows.

The CLI targets Linux, macOS, and Windows and requires Go 1.26 or newer to build
from source.

## Build

```sh
make build
./bin/dh version
```

Run the test suite with `make test` and format Go sources with `make fmt`.

## Project decisions

- Go module: `github.com/dolthub/cli`
- DoltHub web origin: `https://www.dolthub.com`
- REST API: v2, rooted at `https://www.dolthub.com/api/v2/`
- API contract: DoltHub's published OpenAPI 3.1 specification
- Config: JSON at `dh/config.json` beneath the directory returned by
  `os.UserConfigDir()`

## Development authentication

Browser authentication uses DoltHub OAuth 2.0 authorization code flow with
PKCE. Until release builds carry the shared, DoltHub-owned public client ID,
provide a development OAuth application's public client ID at runtime:

```sh
DH_HOST=dev.dolthub.com \
DH_OAUTH_CLIENT_ID=your-public-client-id \
dh auth login
```

The OAuth application must register
`http://localhost:53682/callback` as a redirect URI and support the
`api_read_write` scope. `DH_OAUTH_CLIENT_ID` is public application metadata;
never configure a client secret in `dh`. `DH_HOST` selects the matching web and
API v2 origin, and `DH_TOKEN` remains available for non-persisted automation.

`dh` prefers the operating system credential manager. If it is unavailable,
login falls back to an unencrypted `credentials.json` in the `dh` user config
directory and prints a warning with the exact path. On systems with POSIX file
permissions, the directory is restricted to `0700` and the file to `0600`.
