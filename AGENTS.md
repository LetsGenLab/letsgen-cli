# Public repository instructions

- This repository is public. Never include internal ticket numbers or identifiers
  in commit messages, branch names, tag messages, release notes, pull requests,
  public documentation, examples, or tracked files. Use plain descriptions of
  the change. Keep internal tracking references in private systems only.
- Development defaults to `develop`. Promote reviewed changes to `main`;
  a semver `v*` tag on a commit belonging to `main` publishes a GitHub release.
  Prerelease tags publish prereleases.
- Preserve unrelated changes. Inspect Git status before edits and commits.
- Run `go test -race ./...` and `go vet ./...` for Go changes. Before releases,
  build all six platform archives and run the archive and installer checks.
- Never put credentials in tracked files, commit messages, logs, or command
  arguments. Browser login creates a revocable 30-day personal API key.
- The preview defaults to production. Production actions and paid generation
  require explicit user authorization; a release does not grant either.
- Stop task-owned callback listeners, servers, test processes, and browser
  sessions when finished. Remove temporary credentials and revoke test keys.
