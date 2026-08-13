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

Authentication implementation is intentionally deferred until the foundational
command and application lifecycle are established and the DoltHub OAuth client
registration details are confirmed.

See [SKELETON.md](SKELETON.md) for the architecture and phased roadmap.
