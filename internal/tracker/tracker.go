package tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/saaga0h/themis/internal/labels"
)

// maxResponseBytes bounds how much of an HTTP response body is read, guarding
// against unbounded memory use from an oversized or malicious response.
var maxResponseBytes int64 = 10 * 1024 * 1024

// IssueData holds the data extracted from an issue tracker.
type IssueData struct {
	Number int
	Title  string
	Body   string
	Labels []string
	URL    string
	Ref    string
}

// IssueItemLabel is a label entry in a list-issues API response.
type IssueItemLabel struct {
	Name string `json:"name"`
}

// IssueItem is the raw API shape for a list-issues response.
type IssueItem struct {
	Number int              `json:"number"`
	Title  string           `json:"title"`
	Body   string           `json:"body"`
	Labels []IssueItemLabel `json:"labels"`
}

// Fetcher retrieves a single issue from an issue tracker.
type Fetcher interface {
	Fetch(ctx context.Context, number int) (*IssueData, error)
}

var checkboxRE = regexp.MustCompile(`(?m)^- \[[ xX]\] (.+)$`)

// fence is a top-level (outermost) fenced code block found in an issue body.
// A fence nested inside a longer outer fence — e.g. a ```check block shown
// literally inside a ````markdown example — is not reported as its own
// fence; it is content of the outer one, per CommonMark fence-length rules.
type fence struct {
	start, end               int // byte span of the whole fence, opening line through closing line
	contentStart, contentEnd int // byte span of the fence's inner content
	info                     string
}

// topLevelFences scans body for fenced code blocks (``` or ~~~, three or
// more marker characters) and returns only the outermost ones in source
// order. It is what makes ParseCheckboxes, ParseCheckBlocks, and
// ValidateDestructiveChecks fence-aware: checkbox and check-block syntax
// shown inside a fenced example must not be mistaken for real ACs or check
// directives.
func topLevelFences(body string) []fence {
	lines := strings.Split(body, "\n")
	offsets := make([]int, len(lines)+1)
	off := 0
	for i, l := range lines {
		offsets[i] = off
		off += len(l)
		if i != len(lines)-1 {
			off++ // account for the "\n" strings.Split consumed
		}
	}
	offsets[len(lines)] = off

	var fences []fence
	i := 0
	for i < len(lines) {
		indent, rest := leadingIndent(lines[i])
		if indent <= 3 {
			ch, runLen := fenceMarker(rest)
			if runLen >= 3 {
				info := strings.TrimSpace(rest[runLen:])
				closeIdx := -1
				for j := i + 1; j < len(lines); j++ {
					cIndent, cRest := leadingIndent(lines[j])
					if cIndent > 3 {
						continue
					}
					cCh, cRunLen := fenceMarker(cRest)
					if cRunLen >= runLen && cCh == ch && strings.TrimSpace(cRest[cRunLen:]) == "" {
						closeIdx = j
						break
					}
				}
				var endOffset, contentEnd, nextI int
				if closeIdx == -1 {
					endOffset = len(body)
					contentEnd = len(body)
					nextI = len(lines)
				} else {
					endOffset = offsets[closeIdx] + len(lines[closeIdx])
					contentEnd = offsets[closeIdx]
					nextI = closeIdx + 1
				}
				fences = append(fences, fence{
					start:        offsets[i],
					end:          endOffset,
					contentStart: offsets[i+1],
					contentEnd:   contentEnd,
					info:         info,
				})
				i = nextI
				continue
			}
		}
		i++
	}
	return fences
}

// leadingIndent splits off up to a line's leading spaces, per CommonMark's
// rule that a fence marker may be indented at most three spaces.
func leadingIndent(line string) (int, string) {
	n := 0
	for n < len(line) && line[n] == ' ' {
		n++
	}
	return n, line[n:]
}

// fenceMarker reports the run of leading backticks or tildes at the start of
// s, if any (a run shorter than 3 is not a fence marker and is reported as
// such by the caller checking the returned length).
func fenceMarker(s string) (byte, int) {
	if len(s) == 0 || (s[0] != '`' && s[0] != '~') {
		return 0, 0
	}
	ch := s[0]
	n := 0
	for n < len(s) && s[n] == ch {
		n++
	}
	return ch, n
}

// inFence reports whether byte offset pos falls inside any fence in fences.
// It binary-searches fences, relying on topLevelFences returning them in
// strictly ascending, non-overlapping order by start.
func inFence(fences []fence, pos int) bool {
	i := sort.Search(len(fences), func(i int) bool { return fences[i].start > pos })
	return i > 0 && pos < fences[i-1].end
}

// ParseCheckboxes extracts checkbox items from a markdown body. Both
// unchecked (- [ ]) and checked (- [x]) items are returned. Checkbox lines
// inside a fenced code block (e.g. an example showing AC syntax) are not
// real ACs and are ignored.
func ParseCheckboxes(body string) []string {
	fences := topLevelFences(body)
	matches := checkboxRE.FindAllStringSubmatchIndex(body, -1)
	items := make([]string, 0, len(matches))
	for _, m := range matches {
		if inFence(fences, m[0]) {
			continue
		}
		items = append(items, strings.TrimSpace(body[m[2]:m[3]]))
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

// ParseCheckBlocks extracts each top-level fenced ```check ... ``` block's
// inner command from a markdown issue body, trimmed, in source order. These
// are the issue-declared negative/placement/delegation-AC verifications
// described in skills/issue-writer/SKILL.md: the factory appends each to the
// Green Gate for that run only — issue checks are never committed and never
// touch the project's .themis/workflow.yaml verify contract. A check fence
// shown nested inside a longer outer fence (e.g. an example) is content of
// that outer fence, not a real check directive, and is ignored.
func ParseCheckBlocks(body string) []string {
	fences := topLevelFences(body)
	var blocks []string
	for _, f := range fences {
		if f.info != "check" {
			continue
		}
		blocks = append(blocks, strings.TrimSpace(body[f.contentStart:f.contentEnd]))
	}
	return blocks
}

// destructiveACRE matches the AC phrasings skills/issue-writer/SKILL.md
// prescribes for negative/absence ACs: "No X remains", "X no longer exists",
// and "Removed X".
var destructiveACRE = regexp.MustCompile(`(?i)^no\b.*\bremains?\b|\bno longer exists?\b|^removed\b`)

// IsDestructiveAC reports whether ac describes a destructive/negative AC — one
// asserting that something must no longer exist after the change (e.g. "No
// GiteaQuerier struct remains in cmd/themis", "Removed hardcoded Finna config
// from GetSources handler"). Destructive ACs require a paired check block; see
// ValidateDestructiveChecks.
func IsDestructiveAC(ac string) bool {
	return destructiveACRE.MatchString(strings.TrimSpace(ac))
}

// ValidateDestructiveChecks returns an error naming the offending AC when body
// contains a destructive AC (per IsDestructiveAC) with no check block
// positioned in its own span — from that AC's checkbox up to the next
// destructive AC (or end of body), so a behavioural AC paired after it does not
// orphan the check. Returns nil when every destructive AC has a
// check block in its own span, or when body has no destructive ACs at all.
// Pairing is by position rather than by count: two check blocks stacked after
// one destructive AC do not satisfy a later destructive AC that has none of
// its own, per the "one check block per negative/placement/delegation AC"
// rule in skills/issue-writer/SKILL.md.
func ValidateDestructiveChecks(body string) error {
	fences := topLevelFences(body)

	var acMatches [][]int
	for _, m := range checkboxRE.FindAllStringSubmatchIndex(body, -1) {
		if inFence(fences, m[0]) {
			continue
		}
		acMatches = append(acMatches, m)
	}
	if len(acMatches) == 0 {
		return nil
	}

	var checkBlockStarts []int
	for _, f := range fences {
		if f.info == "check" {
			checkBlockStarts = append(checkBlockStarts, f.start)
		}
	}

	for i, m := range acMatches {
		ac := strings.TrimSpace(body[m[2]:m[3]])
		if !IsDestructiveAC(ac) {
			continue
		}
		spanStart := m[0]
		// The span runs to the next DESTRUCTIVE AC (or end of body), not the next
		// checkbox of any kind — so a behavioural AC paired after the destructive
		// one does not orphan its check block. Each destructive AC is still bound
		// to a check in its own span, keeping the clustered-checks hole closed.
		spanEnd := len(body)
		for j := i + 1; j < len(acMatches); j++ {
			nextAC := strings.TrimSpace(body[acMatches[j][2]:acMatches[j][3]])
			if IsDestructiveAC(nextAC) {
				spanEnd = acMatches[j][0]
				break
			}
		}
		paired := false
		for _, cb := range checkBlockStarts {
			if cb >= spanStart && cb < spanEnd {
				paired = true
				break
			}
		}
		if !paired {
			return fmt.Errorf("destructive AC %q has no accompanying check block", ac)
		}
	}
	return nil
}

// ghIssue mirrors the JSON shape returned by `gh issue view --json`.
type ghIssue struct {
	Number int              `json:"number"`
	Title  string           `json:"title"`
	Body   string           `json:"body"`
	State  string           `json:"state"`
	URL    string           `json:"url"`
	Ref    string           `json:"ref"`
	Labels []IssueItemLabel `json:"labels"`
}

// ParseGitHubJSON parses the JSON output of `gh issue view --json`.
func ParseGitHubJSON(data []byte) (*IssueData, error) {
	var gh ghIssue
	if err := json.Unmarshal(data, &gh); err != nil {
		return nil, fmt.Errorf("parsing gh JSON: %w", err)
	}
	return &IssueData{
		Number: gh.Number,
		Title:  gh.Title,
		Body:   gh.Body,
		Labels: extractLabelNames(gh.Labels),
		URL:    gh.URL,
		Ref:    gh.Ref,
	}, nil
}

// GitHubFetcher fetches issues via the `gh` CLI.
type GitHubFetcher struct{}

// Fetch retrieves issue number via `gh issue view --json`.
func (g *GitHubFetcher) Fetch(ctx context.Context, number int) (*IssueData, error) {
	out, err := exec.CommandContext(ctx, "gh", "issue", "view",
		fmt.Sprintf("%d", number),
		"--json", "number,title,body,labels,state,url",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh issue view %d: %w", number, err)
	}
	return ParseGitHubJSON(out)
}

// GiteaFetcher fetches issues from a Gitea instance via its REST API.
type GiteaFetcher struct {
	owner   string
	repo    string
	apiBase string
	token   string
	client  *http.Client
}

// NewGiteaFetcher creates a GiteaFetcher for the given repo.
func NewGiteaFetcher(owner, repo, apiBase, token string, timeout time.Duration) *GiteaFetcher {
	return &GiteaFetcher{
		owner:   owner,
		repo:    repo,
		apiBase: strings.TrimRight(apiBase, "/"),
		token:   token,
		client:  &http.Client{Timeout: timeout},
	}
}

// giteaIssue mirrors the Gitea API issue shape.
type giteaIssue struct {
	Number  int              `json:"number"`
	Title   string           `json:"title"`
	Body    string           `json:"body"`
	HTMLURL string           `json:"html_url"`
	Ref     string           `json:"ref"`
	Labels  []IssueItemLabel `json:"labels"`
}

// Fetch retrieves issue number from Gitea.
func (g *GiteaFetcher) Fetch(ctx context.Context, number int) (*IssueData, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%d", g.apiBase, g.owner, g.repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	if g.token != "" {
		req.Header.Set("Authorization", "token "+g.token)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		return nil, fmt.Errorf("Gitea API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var gi giteaIssue
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&gi); err != nil {
		return nil, fmt.Errorf("decoding Gitea response: %w", err)
	}

	return &IssueData{
		Number: gi.Number,
		Title:  gi.Title,
		Body:   gi.Body,
		Labels: extractLabelNames(gi.Labels),
		URL:    gi.HTMLURL,
		Ref:    gi.Ref,
	}, nil
}

// ParseIssueItems converts a slice of IssueItem into a slice of *IssueData.
func ParseIssueItems(items []IssueItem) []*IssueData {
	result := make([]*IssueData, 0, len(items))
	for _, item := range items {
		result = append(result, &IssueData{
			Number: item.Number,
			Title:  item.Title,
			Body:   item.Body,
			Labels: extractLabelNames(item.Labels),
		})
	}
	return result
}

func extractLabelNames(labels []IssueItemLabel) []string {
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		names = append(names, l.Name)
	}
	return names
}

// NewFetcher returns a Fetcher for the given provider.
// provider must be "github" or "gitea". owner, repo, and timeout are required for Gitea.
func NewFetcher(provider, owner, repo, apiBase, token string, timeout time.Duration) (Fetcher, error) {
	switch provider {
	case "github":
		return &GitHubFetcher{}, nil
	case "gitea":
		return NewGiteaFetcher(owner, repo, apiBase, token, timeout), nil
	default:
		return nil, fmt.Errorf("unknown provider %q (must be github or gitea)", provider)
	}
}

const querierPageSize = 50

// GiteaQuerier implements an issue querier against the Gitea REST API.
type GiteaQuerier struct {
	owner   string
	repo    string
	apiBase string
	token   string
	client  *http.Client
}

// NewGiteaQuerier creates a GiteaQuerier for the given repo with the specified HTTP timeout.
func NewGiteaQuerier(owner, repo, apiBase, token string, timeout time.Duration) *GiteaQuerier {
	return &GiteaQuerier{
		owner:   owner,
		repo:    repo,
		apiBase: strings.TrimRight(apiBase, "/"),
		token:   token,
		client:  &http.Client{Timeout: timeout},
	}
}

func (q *GiteaQuerier) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if q.token != "" {
		req.Header.Set("Authorization", "token "+q.token)
	}
	return q.client.Do(req)
}

// ListReadyIssues returns all open issues labelled ready-for-agent.
func (q *GiteaQuerier) ListReadyIssues(ctx context.Context) ([]*IssueData, error) {
	var all []IssueItem
	for page := 1; ; page++ {
		pageURL := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues?state=open&type=issues&limit=%d&page=%d&labels="+labels.ReadyForAgent,
			q.apiBase, q.owner, q.repo, querierPageSize, page)
		resp, err := q.get(ctx, pageURL)
		if err != nil {
			return nil, fmt.Errorf("listing issues page %d: %w", page, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("gitea API returned %d", resp.StatusCode)
		}
		var items []IssueItem
		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&items)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decoding issues page %d: %w", page, decodeErr)
		}
		all = append(all, items...)
		if len(items) < querierPageSize {
			break
		}
	}
	return ParseIssueItems(all), nil
}

// IsOpen reports whether issue number is in the open state.
func (q *GiteaQuerier) IsOpen(ctx context.Context, number int) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%d", q.apiBase, q.owner, q.repo, number)
	resp, err := q.get(ctx, url)
	if err != nil {
		return false, fmt.Errorf("checking issue #%d: %w", number, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("gitea API returned %d for issue #%d", resp.StatusCode, number)
	}
	var issue struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&issue); err != nil {
		return false, fmt.Errorf("decoding issue #%d: %w", number, err)
	}
	return issue.State == "open", nil
}

// GitHubQuerier implements an issue querier using the gh CLI.
type GitHubQuerier struct{}

// NewGitHubQuerier creates a GitHubQuerier.
func NewGitHubQuerier() *GitHubQuerier {
	return &GitHubQuerier{}
}

// ListReadyIssues returns all open issues labelled ready-for-agent via the gh CLI.
func (q *GitHubQuerier) ListReadyIssues(ctx context.Context) ([]*IssueData, error) {
	out, err := exec.CommandContext(ctx, "gh", "issue", "list",
		"--label", labels.ReadyForAgent,
		"--state", "open",
		"--json", "number,title,body,labels",
		"--limit", "1000",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh issue list: %w", err)
	}
	var items []IssueItem
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}
	return ParseIssueItems(items), nil
}

// IsOpen reports whether issue number is in the open state via the gh CLI.
func (q *GitHubQuerier) IsOpen(ctx context.Context, number int) (bool, error) {
	out, err := exec.CommandContext(ctx, "gh", "issue", "view",
		fmt.Sprintf("%d", number),
		"--json", "state",
	).Output()
	if err != nil {
		return false, fmt.Errorf("gh issue view %d: %w", number, err)
	}
	var result struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return false, fmt.Errorf("parsing gh output: %w", err)
	}
	return strings.ToLower(result.State) == "open", nil
}
