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
```

Non-interactive download for scripts:

```bash
gh attachment download --issue 123 --all
gh attachment download --pr 456 --all --output ./downloads
```

`--output` defaults to the current directory.

## Repository selection

Use `--repo owner/repo`, pass a GitHub issue/PR URL, or run the command inside a repository where `gh repo view` works.
