---
name: letsgen
description: Use the Lets Gen CLI to discover models, upload references, generate media and recover durable tasks.
---

# Lets Gen CLI v0.1

Run `letsgen help` first. This preview defaults to production. An explicit
`--origin https://letsgen.app` selects production; never change environments
without user authorization. Credentials are isolated by origin.

Use `letsgen --json models list --kind image` and `models inspect MODEL`
before choosing settings. Capabilities and availability come from the server;
baseGems is not an exact quote. Model parameter schemas may be incomplete;
do not invent parameters. Use `--parameters '{"ratio":"1:1"}'` for supported
settings and `--reference OWNED_ASSET_ID` for each uploaded reference.

Ask the user for explicit spending authorization and a numeric budget before
any paid generation. Pass `--max-gems N` for each approved request. This is
an admission limit, not a total session budget. Track chargedGems and remaining
budget across tasks. Login, discovery, polling, downloads and `--dry-run` do
not authorize generation. Never expose credentials in arguments or output.

Preview with `generate image --model MODEL --prompt TEXT --dry-run --json`.
Uploads are explicit via `assets upload FILE`; a dry run never uploads.
For video and audio use `generate video` or `generate audio --operation
speech|music|voice_clone`; speech/clone requires an authorized voiceProfileId
in parameters. Use `voices list` to discover owned voices.

Every generation saves its request identity before submission. On uncertain
responses use `requests list` and `requests retry ID` with the original origin
and credential. Never create a fresh identity to recover an ambiguous request.
`--async` returns a durable task; use `tasks get ID`, `tasks wait ID` and
`tasks download ID --output DIR`. Timeouts do not cancel server tasks.
Downloads refresh task URLs and never trigger another generation or overwrite
existing files. Partial tasks can have successful outputs; report partial status.

For automation use LETSGEN_API_KEY. For interactive login use `auth login`
or `auth login --api-key` (hidden prompt). `auth logout` revokes the saved key.
Human-readable diagnostics are on stderr; `--json` keeps stdout parseable.
