# gh-attachment

GitHub CLI extension to list and download files attached to issues and pull requests.

```bash
gh attachment list --issue 123
gh attachment list --pr 456
gh attachment download --issue 123
gh attachment download --pr 456 --all --output ./downloads
gh attachment download https://github.com/owner/repo/pull/456
```

`list` only prints attachments. `download` opens an interactive picker unless `--all` is set.

## Install

From a published repository:

```bash
gh extension install OWNER/gh-attachment
```

Local development:

```bash
go build -o gh-attachment .
gh extension install .
```

## Usage

```bash
gh attachment list (--issue N | --pr N | URL) [--repo owner/repo]
gh attachment download (--issue N | --pr N | URL) [--repo owner/repo] [--all] [--output DIR]
```

If `--repo` is omitted, the current repository is detected with `gh repo view`.

The URL form supports:

- `https://github.com/owner/repo/issues/123`
- `https://github.com/owner/repo/pull/456`

## Authentication

The extension does not manage credentials. It uses the existing GitHub CLI setup:

```bash
gh auth login
# or GH_TOKEN=...
```

Downloads fail if GitHub returns an HTML login/error page instead of the file.

## Interactive keys

- `↑` / `↓` move
- `Space` toggle selected file
- `A` select all
- `Enter` download selected files
- `Esc` cancel
