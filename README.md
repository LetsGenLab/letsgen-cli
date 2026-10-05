# Lets Gen CLI

Make images, videos, and audio with [Lets Gen](https://letsgen.app), from your terminal.

Describe what you want, pick a model, and get the result as a link or a file
saved to your computer. Install one executable and sign in through your
browser. No Go, Python, Node.js, or API key to copy and paste.

**Preview:** v0.1.0-alpha.1 uses
[letsgen.app](https://letsgen.app). You need production access.

```sh
letsgen auth login
letsgen models list --kind image
letsgen generate image --model MODEL_ID --prompt 'A red fox in the snow' --max-gems 10
```

Replace `MODEL_ID` with an ID from the model list.

## What you can do

- **Make images, videos, and audio.** Choose a model and describe the result.
- **Use your own images as references.** Upload a file once and reuse its asset ID.
- **Discover models and voices.** See available models and their supported settings.
- **Keep the results.** Download finished outputs straight into a folder.
- **Follow your jobs.** Check a task, wait for completion, or recover an interrupted request.
- **Use it from scripts and agents.** Get JSON output and preview requests without spending Gems.

## Contents

- [Install](#install) — [macOS and Linux](#macos-and-linux) ·
  [Windows](#windows) · [Manual download](#manual-download) · [Upgrading](#upgrading)
- [Getting started](#getting-started)
- [Generating images, video, and audio](#generating-images-video-and-audio)
- [Finding models and voices](#finding-models-and-voices)
- [Jobs and uploads](#jobs-and-uploads)
- [Scripting and agents](#scripting-and-agents)
- [Uninstall](#uninstall)
- [Support](#support) · [License](#license)

## Install

### macOS and Linux

Copy and paste into your terminal:

```sh
curl -fsSL https://github.com/LetsGenLab/letsgen-cli/releases/download/v0.1.0-alpha.1/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
letsgen version
```

The installer chooses the right download, verifies its SHA-256 checksum, and
installs to `~/.local/bin`. No administrator access is needed.
The PATH command applies to this terminal. Add the same `export PATH` line
to your shell profile to use `letsgen` in new terminals.

To choose another install directory:

```sh
curl -fsSL https://github.com/LetsGenLab/letsgen-cli/releases/download/v0.1.0-alpha.1/install.sh \
  | LETSGEN_INSTALL_DIR="$HOME/bin" sh
```

### Windows

Copy and paste into PowerShell:

```powershell
irm https://github.com/LetsGenLab/letsgen-cli/releases/download/v0.1.0-alpha.1/install.ps1 | iex
$env:Path = "$env:LOCALAPPDATA\Programs\LetsGen;$env:Path"
letsgen version
```

Installs to `%LOCALAPPDATA%\Programs\LetsGen` without administrator access.
The PATH command applies to this terminal. To use `letsgen` in new terminals,
add that directory to your user Path in Windows Environment Variables.
Native Windows execution has not yet been verified.

### Manual download

Download your archive from the
[release page](https://github.com/LetsGenLab/letsgen-cli/releases/tag/v0.1.0-alpha.1),
extract it, and put `letsgen` (or `letsgen.exe`) in a directory on your PATH.

| Platform | Archive |
| --- | --- |
| macOS, Apple silicon | `letsgen_v0.1.0-alpha.1_darwin_arm64.tar.gz` |
| macOS, Intel | `letsgen_v0.1.0-alpha.1_darwin_amd64.tar.gz` |
| Linux, x86-64 | `letsgen_v0.1.0-alpha.1_linux_amd64.tar.gz` |
| Linux, arm64 | `letsgen_v0.1.0-alpha.1_linux_arm64.tar.gz` |
| Windows, x86-64 | `letsgen_v0.1.0-alpha.1_windows_amd64.zip` |
| Windows, arm64 | `letsgen_v0.1.0-alpha.1_windows_arm64.zip` |

The release includes `checksums.txt` for verifying manual downloads.
The installers verify checksums automatically.

### Upgrading

Run the installer from the new release again. It replaces the existing
executable in the same directory.

## Getting started

```sh
letsgen auth login                  # opens your browser
letsgen auth status                 # checks access, scopes, and Gem usage
```

Approve access in your browser. You can choose read-only access and a monthly
Gem limit. Login saves a 30-day personal API key for you. Sign in again when
it expires; `auth logout` revokes it and removes the local credential.

```sh
# browse without uploading or generating
letsgen auth login --read-only

# choose a monthly Gem limit; 0 disables spending
letsgen auth login --monthly-gem-cap 100
letsgen auth login --monthly-gem-cap 0

# remote terminal: enter an API key at the hidden prompt
letsgen auth login --api-key
```

`--no-browser` prints the login URL instead of opening it. The browser must
be able to reach this terminal's loopback address.

## Generating images, video, and audio

Generation spends real Gems, including in production. Set `--max-gems` to the
most you want to spend on each request. Your monthly key limit also applies.

```sh
# text to image
letsgen generate image --model MODEL_ID --prompt 'A red fox in the snow' --max-gems 10

# save the result to a folder
letsgen generate image --model MODEL_ID --prompt 'A neon city' --max-gems 10 --output ./out

# model-specific image settings
letsgen generate image --model MODEL_ID --prompt 'A mountain lake' \
  --parameters '{"ratio":"1:1"}' --max-gems 10

# text to video
letsgen generate video --model MODEL_ID --prompt 'A paper boat drifting in the rain' \
  --max-gems 50 --async

# speech: choose an authorized VOICE_ID from voices list
letsgen generate audio --model letsgen-voice --operation speech --prompt 'Hello' \
  --parameters '{"voiceProfileId":"VOICE_ID"}' --max-gems 10 --output ./out
```

Choose a model that supports the media type and parameters.
Audio also supports `--operation music` and `--operation voice_clone`.

Each command waits for completion and prints the result, with a default
timeout of 10 minutes. `--timeout 20m` changes the wait.
`--async` submits and returns a task immediately; use `tasks wait` to collect
it later. A timeout does not cancel the server task.

## Finding models and voices

```sh
letsgen models list --kind image    # image models
letsgen models list --kind video    # video models
letsgen models list --kind audio    # audio models
letsgen models inspect MODEL_ID     # supported settings and metadata

letsgen voices list --scope mine
letsgen voices list --scope explore --language en
```

Model IDs come from the current catalog. Inspect a model before choosing
settings. Some models have incomplete parameter details; `baseGems` is a
base price, not an exact cost quote.

## Jobs and uploads

```sh
# check a job, wait for completion, and save its outputs
letsgen tasks get TASK_ID
letsgen tasks wait TASK_ID --timeout 10m
letsgen tasks download TASK_ID --output ./out

# upload a reference image; copy the returned asset ID
letsgen assets upload ./reference.png

# use that image in a generation
letsgen generate image --model MODEL_ID --prompt 'Make this scene snowy' \
  --reference ASSET_ID --max-gems 10 --output ./out

# recover a submission interrupted before it returned a task ID
letsgen requests list
letsgen requests retry REQUEST_ID --async
```

Replace `TASK_ID`, `ASSET_ID`, and `REQUEST_ID` with IDs from the corresponding
command output. Repeat `--reference` to supply multiple images.

Downloading never creates another generation or overwrites existing files.
A partial task may still have completed outputs. Request recovery reuses the
saved request identity, original payload, API origin, and credential.

## Scripting and agents

- **`--json`** returns machine-readable output on stdout. Progress and errors go to stderr.
- **`--dry-run`** previews a generation request without API calls, uploads, or spending.
- **`--async`** submits generation and returns its task without waiting.

```sh
# rehearse a generation without spending
letsgen --json generate image --model MODEL_ID --prompt 'A red fox' --dry-run

# inspect models and jobs as JSON
letsgen --json models list --kind image
letsgen --json tasks get TASK_ID

# read or install the bundled agent instructions
letsgen skills list
letsgen skills show
letsgen skills install --target codex
letsgen skills install --target claude
```

Agents must obtain a numeric budget and explicit permission before paid
generation, then use `--max-gems` on each approved request. Recover uncertain
submissions with `requests retry` instead of starting a new request.

For automation, provide `LETSGEN_API_KEY` securely in the environment. It
overrides saved credentials; never put a key in command arguments.
`LETSGEN_CONFIG_DIR` overrides credential and request storage.
`--origin URL` or `LETSGEN_API_ORIGIN` selects an API environment.
Credentials are stored separately for each origin.

Commands exit nonzero on failure: 1 request/local error, 2 invalid usage,
3 authentication/access, 4 timeout/interruption, 5 failed task/no outputs.

Run `letsgen help` for the full command list.

## Uninstall

First revoke your saved credential:

```sh
letsgen auth logout
```

On macOS and Linux, remove the default installation:

```sh
rm "$HOME/.local/bin/letsgen" "$HOME/.local/bin/letsgen.LICENSE"
```

To also remove saved request history and configuration:

```sh
# macOS
rm -rf "$HOME/Library/Application Support/letsgen"

# Linux
rm -rf "${XDG_CONFIG_HOME:-$HOME/.config}/letsgen"
```

On Windows, after signing out, run in PowerShell:

```powershell
Remove-Item "$env:LOCALAPPDATA\Programs\LetsGen" -Recurse
Remove-Item "$env:APPDATA\letsgen" -Recurse
```

Remove the install directory from your PATH. If you chose custom install or
configuration directories, remove those instead.

## Support

Bugs and feature requests: [open an issue](https://github.com/LetsGenLab/letsgen-cli/issues).

Include the output of `letsgen version` and the command that failed.
Do not include credentials.

## License

[MIT](LICENSE).

Contributors: [development.md](development.md).
