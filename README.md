# Lets Gen CLI

A small Go client for the Lets Gen public API. No Node.js, agent runtime or
provider credentials are needed. The server owns authorization, moderation,
pricing, Gem reservations and generation. Canvas can call the same executable
in a future integration.

This is a **local v0.1.0 preview**, prepared for review before publication at
`LetsGenLab/letsgen-cli`. Source, tags, archives and installers have not been
published. The preview defaults to `https://letsgen.app`.

## Build and try

```sh
go build -trimpath -ldflags '-s -w' -o bin/letsgen ./cmd/letsgen
./bin/letsgen help
./bin/letsgen auth login
./bin/letsgen --json models list --kind image
./bin/letsgen models inspect letsgen-art
./bin/letsgen generate image --model letsgen-art --prompt 'An adult adventurer' --dry-run
```

Browser login requires the additive CLI login support in the main Worker.
It opens a consent page, offers a monthly cap/read-only access, and stores a
30-day personal key. `auth logout` revokes it. For remote terminals use
`auth login --api-key` (hidden prompt or stdin); browser login offers
`--no-browser` when the browser can reach this terminal's loopback listener.
Device login and refresh tokens are not included.

For automation set `LETSGEN_API_KEY` securely in the environment. Never pass
keys as arguments. Credentials are separated by API origin and stored in the
user config directory (`LETSGEN_CONFIG_DIR` overrides it). Files use mode 0600
and directories 0700 on Unix; Windows uses the user's filesystem ACLs.

## Generate, recover, download

Generation spends real Gems, including in production. Obtain spending approval
before executing these examples. `--max-gems` is a per-request admission
limit; the consented key cap covers outstanding reservations and settled
charges across the UTC calendar month.

```sh
letsgen assets upload ./reference.png
letsgen --json generate image --model letsgen-art --prompt 'An adult adventurer' \
  --parameters '{"ratio":"1:1"}' --reference asset_OWNED --max-gems 10 --async
letsgen requests list
letsgen requests retry REQUEST_ID --async
letsgen tasks get TASK_ID
letsgen tasks wait TASK_ID --timeout 10m
letsgen tasks download TASK_ID --output ./out
letsgen voices list --scope explore
letsgen generate audio --model letsgen-voice --operation speech --prompt 'Hello' \
  --parameters '{"voiceProfileId":"voice_OWNED"}' --max-gems 10 --dry-run
```

Every submission persists its request ID **before** the POST. Ambiguous
responses are never automatically resubmitted with a fresh ID. `requests retry`
reuses the exact body, original origin and original credential. A known task
is read instead of submitted again. A timeout returns its current task on
stdout and recovery guidance on stderr; the server keeps processing.

Default generation waits up to 10 minutes. `--async` returns after submission;
`--output DIR` downloads successful outputs after waiting. Partial tasks retain
their partial status. Downloading refreshes task URLs and never regenerates or
overwrites existing files. Output download is bounded to 512 MB per file and
uses a separate client without credentials. API and media redirects are rejected.

`--dry-run` prints the request without network calls, uploads, spending, or
saved request writes. Model capabilities come from the server. Some models
lack complete parameter schemas and video availability; baseGems is not an
exact quote. `models inspect` prints available authoritative metadata rather
than inventing missing fields. Text/extractor generation, voice-reference
creation, device auth, Homebrew and npm wrappers are outside this preview.

`--json` keeps stdout as JSON; progress/errors go to stderr. Exit codes:
0 success, 1 request/local failure, 2 usage, 3 authentication/access,
4 timeout/interruption, 5 failed task/no downloadable outputs.

## Agent skills

```sh
letsgen skills list
letsgen skills show
letsgen skills install --target codex
letsgen skills install --target claude
```

The versioned skill is embedded in the binary. Installation refuses to
overwrite an existing skill. `--path DIR` selects another agent skill root.

## Release preparation

```sh
go test -race ./...
go vet ./...
python3 scripts/release.py v0.1.0
```

This builds macOS/Linux/Windows amd64 and arm64 archives, `checksums.txt`, and
`source.txt` locally. It does not create GitHub Actions, tags, releases or push
anything. After review, create the public repository with this source, select
an open-source license, tag the reviewed commit, and upload the archives plus
checksum/source files to the matching GitHub release.

After publication, download and inspect `install.sh` (macOS/Linux) or
`install.ps1` (Windows) from the reviewed tag. Both fetch version-pinned GitHub
archives and verify SHA-256 before installing. The Unix default is
`~/.local/bin`; override `LETSGEN_INSTALL_DIR` and `LETSGEN_VERSION` as needed.
Windows accepts `-Version` and `-InstallDir`. Checksum verification protects
against corrupt downloads; signed releases are a future improvement.
