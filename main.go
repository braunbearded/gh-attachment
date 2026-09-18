package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type target struct {
	Repo   string
	Kind   string
	Number int
}

type attachment struct {
	URL       string
	Filename  string
	Source    string
	Timestamp string
}

type options struct {
	Issue  int
	PR     int
	Repo   string
	Output string
	All    bool
	URL    string
}

type issueResponse struct {
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

type commentResponse struct {
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}

type issueListResponse struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	State       string    `json:"state"`
	UpdatedAt   string    `json:"updated_at"`
	PullRequest *struct{} `json:"pull_request"`
}

type targetChoice struct {
	Target    target
	Title     string
	State     string
	UpdatedAt string
}

var attachmentRE = regexp.MustCompile(`https://github\.com/user-attachments/files/[0-9]+/[^\s)>'\"]+`)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		usage()
		return nil
	}

	switch args[0] {
	case "list":
		opts, err := parseOptions(args[1:], false)
		if err != nil {
			return err
		}
		t, err := resolveTarget(opts)
		if err != nil {
			return err
		}
		items, err := findAttachments(t)
		if err != nil {
			return err
		}
		printList(t, items)
	case "download":
		opts, err := parseOptions(args[1:], true)
		if err != nil {
			return err
		}
		var t target
		if hasTarget(opts) {
			t, err = resolveTarget(opts)
		} else {
			t, err = chooseDownloadTarget(opts.Repo, opts.All)
			opts.All = false
			if errors.Is(err, errCanceled) {
				fmt.Println("Canceled")
				return nil
			}
		}
		if err != nil {
			return err
		}
		items, err := findAttachments(t)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Printf("No attachments found for %s #%d\n", t.Kind, t.Number)
			return nil
		}

		selected := items
		if !opts.All {
			selected, err = chooseAttachments(t, items)
			if errors.Is(err, errCanceled) {
				fmt.Println("Canceled")
				return nil
			}
			if err != nil {
				return err
			}
		}
		if len(selected) == 0 {
			fmt.Println("No attachments selected")
			return nil
		}
		return downloadAll(selected, opts.Output)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}

	return nil
}

func usage() {
	fmt.Println(`gh attachment - list and download GitHub issue/PR attachments

Usage:
  gh attachment list --issue 123 [--repo owner/repo]
  gh attachment list --pr 456 [--repo owner/repo]
  gh attachment list https://github.com/owner/repo/issues/123
  gh attachment download --issue 123 [--all] [--output DIR]
  gh attachment download --pr 456 [--all] [--output DIR]
  gh attachment download https://github.com/owner/repo/pull/456 [--all]
  gh attachment download [--all] [--repo owner/repo]`)
}

func parseOptions(args []string, withDownloadFlags bool) (options, error) {
	opts := options{Output: "."}
	fs := flag.NewFlagSet("attachment", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&opts.Issue, "issue", 0, "issue number")
	fs.IntVar(&opts.PR, "pr", 0, "pull request number")
	fs.StringVar(&opts.Repo, "repo", "", "repository owner/name")
	if withDownloadFlags {
		fs.BoolVar(&opts.All, "all", false, "download all without prompting")
		fs.StringVar(&opts.Output, "output", ".", "download directory")
	}
	args = moveURLArgLast(args)
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() > 1 {
		return opts, errors.New("expected at most one issue or pull request URL")
	}
	if fs.NArg() == 1 {
		opts.URL = fs.Arg(0)
	}
	return opts, nil
}

func moveURLArgLast(args []string) []string {
	out := make([]string, 0, len(args))
	var urlArg string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "https://github.com/") && urlArg == "" {
			urlArg = a
			continue
		}
		out = append(out, a)
		if a == "--repo" || a == "--output" || a == "--issue" || a == "--pr" {
			i++
			if i < len(args) {
				out = append(out, args[i])
			}
		}
	}
	if urlArg != "" {
		out = append(out, urlArg)
	}
	return out
}

func hasTarget(opts options) bool {
	return opts.URL != "" || opts.Issue != 0 || opts.PR != 0
}

func resolveTarget(opts options) (target, error) {
	var t target
	if opts.URL != "" {
		parsed, err := parseIssueURL(opts.URL)
		if err != nil {
			return t, err
		}
		t = parsed
	}

	if opts.Issue != 0 {
		if t.Number != 0 {
			return t, errors.New("use either a URL or --issue/--pr, not both")
		}
		t.Kind, t.Number = "issue", opts.Issue
	}
	if opts.PR != 0 {
		if t.Number != 0 {
			return t, errors.New("use either --issue or --pr")
		}
		t.Kind, t.Number = "pull request", opts.PR
	}
	if t.Number == 0 {
		return t, errors.New("missing --issue, --pr, or issue/pull URL")
	}

	if opts.Repo != "" {
		if t.Repo != "" && t.Repo != opts.Repo {
			return t, errors.New("URL repo and --repo differ")
		}
		t.Repo = opts.Repo
	}
	if t.Repo == "" {
		repo, err := currentRepo()
		if err != nil {
			return t, err
		}
		t.Repo = repo
	}
	return t, nil
}

func parseIssueURL(raw string) (target, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host != "github.com" {
		return target{}, fmt.Errorf("unsupported URL %q", raw)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 4 || (parts[2] != "issues" && parts[2] != "pull") {
		return target{}, fmt.Errorf("unsupported URL %q", raw)
	}
	n, err := strconv.Atoi(parts[3])
	if err != nil || n <= 0 {
		return target{}, fmt.Errorf("invalid number in URL %q", raw)
	}
	kind := "issue"
	if parts[2] == "pull" {
		kind = "pull request"
	}
	return target{Repo: parts[0] + "/" + parts[1], Kind: kind, Number: n}, nil
}

func currentRepo() (string, error) {
	out, err := exec.Command("gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner").Output()
	if err != nil {
		return "", errors.New("could not detect repository; pass --repo owner/repo")
	}
	return strings.TrimSpace(string(out)), nil
}

func chooseDownloadTarget(repo string, includeClosed bool) (target, error) {
	if repo == "" {
		var err error
		repo, err = currentRepo()
		if err != nil {
			return target{}, err
		}
	}
	choices, err := listTargets(repo, includeClosed)
	if err != nil {
		return target{}, err
	}
	if len(choices) == 0 {
		return target{}, errors.New("no issues or pull requests found")
	}
	return chooseTarget(repo, choices)
}

func listTargets(repo string, includeClosed bool) ([]targetChoice, error) {
	state := "open"
	if includeClosed {
		state = "all"
	}
	data, err := ghAPI("--paginate", "--slurp", fmt.Sprintf("repos/%s/issues?state=%s&per_page=100", repo, state))
	if err != nil {
		return nil, err
	}
	issues, err := parseIssueList(data)
	if err != nil {
		return nil, err
	}
	choices := make([]targetChoice, 0, len(issues))
	for _, issue := range issues {
		kind := "issue"
		if issue.PullRequest != nil {
			kind = "pull request"
		}
		choices = append(choices, targetChoice{
			Target:    target{Repo: repo, Kind: kind, Number: issue.Number},
			Title:     issue.Title,
			State:     issue.State,
			UpdatedAt: issue.UpdatedAt,
		})
	}
	return choices, nil
}

func parseIssueList(data []byte) ([]issueListResponse, error) {
	var pages [][]issueListResponse
	if err := json.Unmarshal(data, &pages); err == nil {
		var issues []issueListResponse
		for _, page := range pages {
			issues = append(issues, page...)
		}
		return issues, nil
	}
	var issues []issueListResponse
	if err := json.Unmarshal(data, &issues); err != nil {
		return nil, err
	}
	return issues, nil
}

func chooseTarget(repo string, choices []targetChoice) (target, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return target{}, errors.New("target selection needs a terminal; pass --issue, --pr, or URL in scripts")
	}
	defer tty.Close()

	restore, err := rawTerminal(tty.Name())
	if err != nil {
		return target{}, err
	}
	defer restore()

	cursor := 0
	for {
		drawTargetPicker(tty, repo, choices, cursor)
		switch readKey(tty) {
		case "up":
			if cursor > 0 {
				cursor--
			}
		case "down":
			if cursor < len(choices)-1 {
				cursor++
			}
		case "enter":
			fmt.Fprint(tty, "\033[H\033[2J")
			return choices[cursor].Target, nil
		case "esc":
			fmt.Fprint(tty, "\033[H\033[2J")
			return target{}, errCanceled
		}
	}
}

func drawTargetPicker(w io.Writer, repo string, choices []targetChoice, cursor int) {
	fmt.Fprint(w, "\033[H\033[2J")
	fmt.Fprintf(w, "Select issue or pull request in %s\n", repo)
	for i, c := range choices {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		kind := "issue"
		if c.Target.Kind == "pull request" {
			kind = "PR"
		}
		fmt.Fprintf(w, "%s#%d %s [%s] %s", prefix, c.Target.Number, kind, c.State, c.Title)
		if ts := shortTime(c.UpdatedAt); ts != "" {
			fmt.Fprintf(w, "  —  %s", ts)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "↑/↓ move  Enter select  Esc cancel")
}

func findAttachments(t target) ([]attachment, error) {
	issueJSON, err := ghAPI(fmt.Sprintf("repos/%s/issues/%d", t.Repo, t.Number))
	if err != nil {
		return nil, err
	}
	var issue issueResponse
	if err := json.Unmarshal(issueJSON, &issue); err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var items []attachment
	add := func(body, source, timestamp string) {
		for _, a := range extractAttachments(body, source, timestamp) {
			if !seen[a.URL] {
				seen[a.URL] = true
				items = append(items, a)
			}
		}
	}
	add(issue.Body, "Issue body", issue.CreatedAt)
	if t.Kind == "pull request" {
		items = nil
		seen = map[string]bool{}
		add(issue.Body, "Pull request body", issue.CreatedAt)
	}

	commentsJSON, err := ghAPI("--paginate", "--slurp", fmt.Sprintf("repos/%s/issues/%d/comments", t.Repo, t.Number))
	if err != nil {
		return nil, err
	}
	comments, err := parseComments(commentsJSON)
	if err != nil {
		return nil, err
	}
	for _, c := range comments {
		who := c.User.Login
		if who == "" {
			who = "unknown"
		}
		add(c.Body, "Comment by "+who, c.CreatedAt)
	}
	return items, nil
}

func parseComments(data []byte) ([]commentResponse, error) {
	var pages [][]commentResponse
	if err := json.Unmarshal(data, &pages); err == nil {
		var comments []commentResponse
		for _, page := range pages {
			comments = append(comments, page...)
		}
		return comments, nil
	}

	var comments []commentResponse
	if err := json.Unmarshal(data, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

func ghAPI(args ...string) ([]byte, error) {
	cmdArgs := append([]string{"api"}, args...)
	out, err := exec.Command("gh", cmdArgs...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh %s failed: %s", strings.Join(cmdArgs, " "), strings.TrimSpace(string(out)))
	}
	return out, nil
}

func extractAttachments(markdown, source, timestamp string) []attachment {
	matches := attachmentRE.FindAllString(markdown, -1)
	items := make([]attachment, 0, len(matches))
	for _, raw := range matches {
		raw = strings.TrimRight(raw, ".,;:")
		filename := filenameFromURL(raw)
		if filename == "" {
			continue
		}
		items = append(items, attachment{URL: raw, Filename: filename, Source: source, Timestamp: timestamp})
	}
	return items
}

func shortTime(raw string) string {
	if raw == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC().Format("2006-01-02 15:04 UTC")
	}
	return raw
}

func filenameFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	name, err := url.PathUnescape(filepath.Base(u.Path))
	if err != nil {
		name = filepath.Base(u.Path)
	}
	if name == "." || name == "/" {
		return ""
	}
	return filepath.Base(name)
}

func printList(t target, items []attachment) {
	fmt.Printf("Attachments for %s #%d:\n\n", t.Kind, t.Number)
	if len(items) == 0 {
		fmt.Println("  none")
		return
	}
	for i, a := range items {
		source := a.Source
		if ts := shortTime(a.Timestamp); ts != "" {
			source += "  —  " + ts
		}
		fmt.Printf("%3d  %-30s %s\n", i+1, a.Filename, source)
	}
}

var errCanceled = errors.New("canceled")

func chooseAttachments(t target, items []attachment) ([]attachment, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, errors.New("interactive selection needs a terminal; use --all in scripts")
	}
	defer tty.Close()

	restore, err := rawTerminal(tty.Name())
	if err != nil {
		return nil, err
	}
	defer restore()

	selected := make([]bool, len(items))
	cursor := 0
	for {
		drawPicker(tty, t, items, selected, cursor)
		switch readKey(tty) {
		case "up":
			if cursor > 0 {
				cursor--
			}
		case "down":
			if cursor < len(items)-1 {
				cursor++
			}
		case "space":
			selected[cursor] = !selected[cursor]
		case "all":
			for i := range selected {
				selected[i] = true
			}
		case "enter":
			fmt.Fprint(tty, "\033[H\033[2J")
			var picked []attachment
			for i, ok := range selected {
				if ok {
					picked = append(picked, items[i])
				}
			}
			return picked, nil
		case "esc":
			fmt.Fprint(tty, "\033[H\033[2J")
			return nil, errCanceled
		}
	}
}

func rawTerminal(device string) (func(), error) {
	state, err := exec.Command("stty", "-F", device, "-g").Output()
	if err != nil {
		return nil, err
	}
	if err := exec.Command("stty", "-F", device, "-icanon", "-echo", "min", "0", "time", "1").Run(); err != nil {
		return nil, err
	}
	return func() { _ = exec.Command("stty", "-F", device, strings.TrimSpace(string(state))).Run() }, nil
}

func drawPicker(w io.Writer, t target, items []attachment, selected []bool, cursor int) {
	fmt.Fprint(w, "\033[H\033[2J")
	fmt.Fprintf(w, "Attachments for %s #%d\n", t.Kind, t.Number)
	count := 0
	for i, a := range items {
		box := "[ ]"
		if selected[i] {
			box = "[x]"
			count++
		}
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		label := a.Filename
		if a.Source != "" {
			label += "  —  " + a.Source
		}
		if ts := shortTime(a.Timestamp); ts != "" {
			label += "  —  " + ts
		}
		fmt.Fprintf(w, "%s%s %s\n", prefix, box, label)
	}
	fmt.Fprintf(w, "Selected: %d/%d  ↑/↓ move  Space select  A all  Enter download  Esc cancel\n", count, len(items))
}

func readKey(r io.Reader) string {
	buf := make([]byte, 3)
	for {
		n, err := r.Read(buf[:1])
		if err != nil || n == 0 {
			continue
		}
		switch buf[0] {
		case 27:
			n, _ := r.Read(buf[1:3])
			if n == 2 && buf[1] == '[' {
				switch buf[2] {
				case 'A':
					return "up"
				case 'B':
					return "down"
				}
			}
			return "esc"
		case ' ':
			return "space"
		case 'a', 'A':
			return "all"
		case '\r', '\n':
			return "enter"
		}
	}
}

func downloadAll(items []attachment, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	token := ghToken()
	used := map[string]bool{}
	for _, a := range items {
		path := uniquePath(dir, a.Filename, used)
		fmt.Printf("Downloading %s\n", filepath.Base(path))
		if err := downloadAttachment(a.URL, path, token); err != nil {
			return err
		}
	}
	return nil
}

func ghToken() string {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func uniquePath(dir, name string, used map[string]bool) string {
	base := filepath.Join(dir, name)
	if !used[base] && !exists(base) {
		used[base] = true
		return base
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if !used[p] && !exists(p) {
			used[p] = true
			return p
		}
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func downloadAttachment(rawURL, dest, token string) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "gh-attachment")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s returned %s", rawURL, resp.Status)
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml") {
		return fmt.Errorf("%s returned HTML, not a file; check gh auth", rawURL)
	}

	reader := bufio.NewReader(resp.Body)
	peek, _ := reader.Peek(512)
	if looksHTML(peek) {
		return fmt.Errorf("%s returned HTML, not a file; check gh auth", rawURL)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".gh-attachment-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	_, copyErr := io.Copy(tmp, reader)
	closeErr := tmp.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmpName, dest)
}

func looksHTML(b []byte) bool {
	b = bytes.TrimSpace(bytes.ToLower(b))
	return bytes.HasPrefix(b, []byte("<!doctype html")) || bytes.HasPrefix(b, []byte("<html"))
}
