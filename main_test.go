package main

import "testing"

func TestParseIssueURL(t *testing.T) {
	tg, err := parseIssueURL("https://github.com/acme/widgets/pull/42")
	if err != nil {
		t.Fatal(err)
	}
	if tg.Repo != "acme/widgets" || tg.Kind != "pull request" || tg.Number != 42 {
		t.Fatalf("unexpected target: %#v", tg)
	}
}

func TestExtractAttachmentsDedupInput(t *testing.T) {
	md := `see [log](https://github.com/user-attachments/files/123/error.log) and <https://github.com/user-attachments/files/456/report%20final.pdf>.`
	items := extractAttachments(md, "Issue body")
	if len(items) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(items))
	}
	if items[0].Filename != "error.log" {
		t.Fatalf("unexpected first filename: %q", items[0].Filename)
	}
	if items[1].Filename != "report final.pdf" {
		t.Fatalf("unexpected second filename: %q", items[1].Filename)
	}
}

func TestParseOptionsAllowsFlagsAfterURL(t *testing.T) {
	opts, err := parseOptions([]string{"https://github.com/acme/widgets/issues/1", "--all", "--output", "dl"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !opts.All || opts.Output != "dl" || opts.URL == "" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParsePaginatedComments(t *testing.T) {
	comments, err := parseComments([]byte(`[[{"body":"one","user":{"login":"a"}}],[{"body":"two","user":{"login":"b"}}]]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 2 || comments[1].Body != "two" || comments[1].User.Login != "b" {
		t.Fatalf("unexpected comments: %#v", comments)
	}
}

func TestLooksHTML(t *testing.T) {
	if !looksHTML([]byte("  <!doctype html><title>Login</title>")) {
		t.Fatal("expected HTML sniff")
	}
}
