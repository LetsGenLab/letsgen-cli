# Development

The CLI is a Go client for the Lets Gen public API. The server owns
authorization, moderation, pricing, Gem reservations, and generation.
Read [AGENTS.md](AGENTS.md) before contributing. This is a public repository;
keep internal tracking identifiers out of all public material.

## Build locally

Install the Go version declared in `go.mod`, then run:

```sh
go build -trimpath -ldflags '-s -w' -o bin/letsgen ./cmd/letsgen
./bin/letsgen help
```

The CLI defaults to production. For internal tests, set `LETSGEN_API_ORIGIN`
in private environment configuration or pass an explicitly approved
`--origin`. Do not publish internal test hostnames. Use read-only access or a
zero-Gem cap for checks without generation. Paid generation requires separate
authorization. Browser login requires the CLI login routes to be deployed in
the selected environment.

## Validate

```sh
go test -race ./...
go vet ./...
python3 scripts/release.py v0.1.0-alpha.1
python3 scripts/verify-release.py dist/v0.1.0-alpha.1
python3 scripts/test-install.py v0.1.0-alpha.1
```

Release preparation produces macOS/Linux/Windows amd64 and arm64 archives,
`checksums.txt`, and `source.txt`. Each archive includes its executable and
MIT license. The Unix installer check executes the native binary and rejects
a corrupted checksum. Windows binaries are cross-built; native Windows
execution is a separate validation step.

The release script never tags, pushes, or publishes.

## Branches and releases

Development defaults to `develop`. GitHub CI validates pushes and pull
requests against `develop` and `main`, including race tests, vet, six-platform
archive checks, and the Unix installer.

Promote the reviewed version to `main`, update the user documentation and
installer defaults, and push an annotated semver tag on that commit.
The release workflow verifies that the tagged commit belongs to `main`,
reruns validation, builds all six archives, and publishes a GitHub release
with checksums, source provenance, and both installers.
Tags with a prerelease suffix produce GitHub prereleases.
A branch push without a tag runs CI without publishing.

Verify the published source SHA, archive checksums, installer behavior, and
release notes. Release material must describe production defaults and any
unverified platform behavior accurately. GitHub publication does not
authorize a production deployment or paid generation.

## Current boundaries

Browser login uses explicit consent and S256 PKCE with a loopback callback.
It issues a revocable 30-day personal API key; there is no device-login or
refresh-token flow. Requests persist their identity before submission, and
recovery reuses the original body and credential.

Model capabilities come from the server; parameter schemas and pricing
metadata may be incomplete. Do not invent missing settings or treat
`baseGems` as an exact quote. Media downloads are limited to 512 MB per file,
use a separate client without credentials, and reject redirects.

Signing, Homebrew, npm wrappers, text/extractor generation, voice-reference
creation, and a Canvas integration are future work.
