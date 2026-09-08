# Releasing dh

## Setup

Create the `dolthub/cli` Docker Hub repository and configure these GitHub Actions
secrets with access to it:

- `DOCKER_HUB_USERNAME`
- `DOCKER_HUB_ACCESS_TOKEN` (image push and repository-description update access)

GitHub releases use the built-in `GITHUB_TOKEN`; no release PAT or version-bump
commit is needed. The release jobs request `contents: write`. Actions are pinned
to commit SHAs. GitHub-hosted Linux, macOS, Windows, and ARM runners are used.

## Release

Merge the release implementation to `main` before using it. Run **Release
DoltHub CLI** from `main`, specifying a stable version such as `0.1.0` or
`v0.1.0`. Prereleases are not supported by this initial workflow.

The workflow resolves `main` once, runs all platform tests and vet plus lint,
builds five archives with `CGO_ENABLED=0` and `-X main.version=VERSION`, and
smoke-tests each archived binary on its native platform. Release checks always
run, independently of the ordinary Go workflow's path filters.

After validation it creates `vVERSION` and a draft release, then uploads:

- `dh-linux-amd64.tar.gz`
- `dh-linux-arm64.tar.gz`
- `dh-darwin-amd64.tar.gz`
- `dh-darwin-arm64.tar.gz`
- `dh-windows-amd64.zip`
- `checksums.txt` (SHA-256)

The Docker workflow checks out that exact tag, builds and pushes
`dolthub/cli:VERSION` for Linux amd64/arm64, verifies both images, updates the
Docker Hub description, and promotes the verified digest to `latest`. Only then
does the parent workflow publish the GitHub release and mark it latest. The
image records its version and source SHA in OCI labels. Go versions come from
the tagged `go.mod`; the production OAuth client ID is included in source.

Release runs are serialized. Do not move release tags. Start stable releases in
increasing version order. Binaries are unsigned in this initial release process;
checksums detect corruption but are not publisher signatures.

## Recovery

- Before the tag exists, retrying the release can select a newer `main` commit.
  Once the tag exists, a retry uses that tag's commit and requires any existing
  release to remain a draft. A retry may replace assets on that draft.
- For asset repair, run **Recover release assets** on `main` with the existing
  version. It builds source and packaging scripts from the tag and reruns the
  platform checks. Existing assets cause upload failure unless `replace` is
  explicitly enabled; use replacement to repair the complete archive/checksum set.
  Immutable GitHub releases cannot have their assets replaced.
- Run **Publish DoltHub CLI Docker image** on `main` to rebuild a versioned image
  from its tag. A manual run does not change `latest` or publish a GitHub release.
  Resume the parent release to complete promotion of a draft release.
- A Docker or final GitHub publication failure can leave a versioned image or
  `latest` visible while the GitHub release is still a draft. Resume the release;
  publication across the two registries is not atomic.
- To roll back consumers, select the previous release's explicit version tag or
  image digest. Do not rewrite an existing Git tag to change released source.

## Local checks

```sh
bash .github/scripts/build_binaries.sh 0.0.0 dist
(cd dist && sha256sum -c checksums.txt)
docker build -f docker/Dockerfile --build-arg DH_VERSION=0.0.0 -t dh-test:local .
docker run --rm dh-test:local version --short
```

**Check release packaging** tests archives and both Docker architectures on
release-related pull requests without publishing or accessing registry secrets.
The first actual release must also verify GitHub/Docker Hub permissions and a
native production browser login; those cannot be established by local tests.
