package main

import (
	"strings"
	"testing"
)

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
	items := extractAttachments(md, "Issue body", "2026-09-18T21:00:00Z")
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

func TestParseIssueList(t *testing.T) {
	issues, err := parseIssueList([]byte(`[[{"number":1,"title":"bug","state":"open","updated_at":"2026-09-18T21:00:00Z"},{"number":2,"title":"change","state":"closed","pull_request":{},"updated_at":"2026-09-18T22:00:00Z"}]]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 || issues[1].PullRequest == nil || issues[1].Title != "change" {
		t.Fatalf("unexpected issues: %#v", issues)
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

func TestDrawTargetPickerIsCompact(t *testing.T) {
	choices := []targetChoice{{Target: target{Kind: "issue", Number: 1}, Title: "bug", State: "open", UpdatedAt: "2026-09-18T21:00:00Z"}}
	var b strings.Builder
	drawTargetPicker(&b, "acme/widgets", choices, 0)
	out := b.String()
	if strings.Contains(out, "\n\n") || !strings.Contains(out, "#1 issue [open] bug") {
		t.Fatalf("unexpected target picker: %q", out)
	}
}

func TestDrawPickerIsCompact(t *testing.T) {
	items := []attachment{{Filename: "one.log", Source: "Issue body", Timestamp: "2026-09-18T21:00:00Z"}, {Filename: "two.zip", Source: "Comment by alice", Timestamp: "2026-09-18T22:00:00Z"}}
	var b strings.Builder
	drawPicker(&b, target{Kind: "issue", Number: 1}, items, []bool{true, false}, 0)
	out := b.String()
	if strings.Contains(out, "\n\n") {
		t.Fatalf("picker contains blank lines: %q", out)
	}
	if !strings.Contains(out, "one.log  —  Issue body  —  2026-09-18") || !strings.Contains(out, "two.zip  —  Comment by alice  —  2026-09-18") {
		t.Fatalf("missing attachment source: %q", out)
	}
	if !strings.Contains(out, "Selected: 1/2") {
		t.Fatalf("missing compact selected count: %q", out)
	}
}

func TestLooksHTML(t *testing.T) {
	if !looksHTML([]byte("  <!doctype html><title>Login</title>")) {
		t.Fatal("expected HTML sniff")
	}
}
