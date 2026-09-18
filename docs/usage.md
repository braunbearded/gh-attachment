# Usage

## List attachments

```bash
gh attachment list --issue 123
gh attachment list --pr 456
gh attachment list --repo owner/repo --issue 123
gh attachment list https://github.com/owner/repo/issues/123
```

Output includes the filename, where it was found, and the issue/comment timestamp.

## Download attachments

Interactive picker:

```bash
gh attachment download --issue 123
gh attachment download --pr 456
gh attachment download
```

Without `--issue`, `--pr`, or URL, open issues and pull requests are shown first. Pick one, then the normal attachment picker opens. Add `--all` in this mode to include closed issues and pull requests too.

Non-interactive download for scripts with an explicit target:

```bash
gh attachment download --issue 123 --all
gh attachment download --pr 456 --all --output ./downloads
```

`--output` defaults to the current directory.

## Repository selection

Use `--repo owner/repo`, pass a GitHub issue/PR URL, or run the command inside a repository where `gh repo view` works.
