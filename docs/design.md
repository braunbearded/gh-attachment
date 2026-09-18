# Design

Version 1 is intentionally small:

- one Go binary for `gh attachment`
- a tiny root `gh-attachment` launcher so `gh extension install OWNER/gh-attachment` works before releases exist
- no Go dependencies
- GitHub data is fetched through `gh api`
- credentials come from `gh auth` / `GH_TOKEN`
- issues and pull requests share the same issue API path

## Attachment detection

Bodies and comments are scanned for URLs matching:

```text
https://github.com/user-attachments/files/<id>/<filename>
```

Duplicates are removed by URL. The saved filename comes from the URL path, not from an HTML response.

## Download safety

Downloads use the `gh auth token` bearer token when available. Before writing the final file, the response is checked so HTML login/error pages are rejected instead of saved as attachments.

## Release

The GitHub Actions workflow builds precompiled extension artifacts when a `v*` tag is pushed. Until a release exists, GitHub CLI installs the root `gh-attachment` launcher and builds the binary locally.

## Non-goals for v1

No upload, delete, JSON output, filtering, history, or custom credential storage.
