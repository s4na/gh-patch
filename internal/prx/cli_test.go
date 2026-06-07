package prx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

const twoThreeSHA = "43fc3d02ea6e854a19e994c75467d8163dde2464c83a12a5c56c59f47f75e253"

type fakeGitHub struct {
	pr               PullRequest
	comments         []Comment
	updatedPRBody    string
	updatedComments  map[int64]string
	createdComment   string
	expectedPRNumber int
	commentsByID     map[int64]Comment
	currentLogin     string
	fail             error
}

func (f *fakeGitHub) CurrentLogin() (string, error) {
	if f.fail != nil {
		return "", f.fail
	}
	if f.currentLogin == "" {
		return "github-actions[bot]", nil
	}
	return f.currentLogin, nil
}

func (f *fakeGitHub) GetPullRequest(number int) (PullRequest, error) {
	if err := f.checkPRNumber(number); err != nil {
		return PullRequest{}, err
	}
	if f.fail != nil {
		return PullRequest{}, f.fail
	}
	return f.pr, nil
}

func (f *fakeGitHub) UpdatePullRequestBody(number int, body string) (PullRequest, error) {
	if err := f.checkPRNumber(number); err != nil {
		return PullRequest{}, err
	}
	if f.fail != nil {
		return PullRequest{}, f.fail
	}
	f.updatedPRBody = body
	f.pr.Body = body
	return f.pr, nil
}

func (f *fakeGitHub) ListComments(number int) ([]Comment, error) {
	if err := f.checkPRNumber(number); err != nil {
		return nil, err
	}
	if f.fail != nil {
		return nil, f.fail
	}
	return f.comments, nil
}

func (f *fakeGitHub) GetComment(id int64) (Comment, error) {
	if f.fail != nil {
		return Comment{}, f.fail
	}
	if f.commentsByID != nil {
		if comment, ok := f.commentsByID[id]; ok {
			return comment, nil
		}
	}
	for _, comment := range f.comments {
		if comment.ID == id {
			return comment, nil
		}
	}
	return Comment{}, errors.New("not found")
}

func (f *fakeGitHub) UpdateComment(id int64, body string) (Comment, error) {
	if f.fail != nil {
		return Comment{}, f.fail
	}
	if f.updatedComments == nil {
		f.updatedComments = map[int64]string{}
	}
	f.updatedComments[id] = body
	return Comment{ID: id, Body: body, URL: "https://example.test/comment"}, nil
}

func (f *fakeGitHub) CreateComment(number int, body string) (Comment, error) {
	if err := f.checkPRNumber(number); err != nil {
		return Comment{}, err
	}
	if f.fail != nil {
		return Comment{}, f.fail
	}
	f.createdComment = body
	return Comment{ID: 999, Body: body, URL: "https://example.test/comment/999"}, nil
}

func (f *fakeGitHub) checkPRNumber(number int) error {
	if f.expectedPRNumber == 0 {
		return nil
	}
	if number != f.expectedPRNumber {
		return fmt.Errorf("unexpected PR number: got %d want %d", number, f.expectedPRNumber)
	}
	return nil
}

func TestBodyReadPlainPrintsMarkerContent(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "<!-- section:start -->\nsection\n<!-- section:end -->"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "read", "123", "--marker", "section", "--plain"}, strings.NewReader(""), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	if stdout.String() != "section\n" {
		t.Fatalf("stdout = %q, want section newline", stdout.String())
	}
}

func TestBodyWriteDryRunPrintsDiffWithoutUpdating(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "<!-- section:start -->\nold content\n<!-- section:end -->", URL: "https://example.test/pr/123"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "write", "123", "--marker", "section", "-", "--dry-run"}, strings.NewReader("new content\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	if gh.updatedPRBody != "" {
		t.Fatalf("dry-run updated PR body: %q", gh.updatedPRBody)
	}
	out := stdout.String()
	for _, want := range []string{"<!-- section:start -->", "op | line | content", " - |    1 | old content", " + |    1 | new content", "<!-- section:end -->"} {
		if !strings.Contains(out, want) {
			t.Fatalf("diff output = %q, want to contain %q", out, want)
		}
	}
}

func TestBodyWriteNoChangesReturnsDedicatedExitCode(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "<!-- section:start -->\nsame\n<!-- section:end -->"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "write", "123", "--marker", "section", "-"}, strings.NewReader("same\n"), &stdout, &stderr, gh)

	if code != ExitNoChanges {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitNoChanges, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "no changes" {
		t.Fatalf("stdout = %q, want no changes", stdout.String())
	}
	if gh.updatedPRBody != "" {
		t.Fatalf("no-op updated PR body: %q", gh.updatedPRBody)
	}
}

func TestBodyWriteMarkerMissingReturnsActionableError(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "plain body"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "write", "123", "--marker", "section", "-"}, strings.NewReader("section\n"), &stdout, &stderr, gh)

	if code != ExitMarkerNotFound {
		t.Fatalf("exit code = %d, want %d", code, ExitMarkerNotFound)
	}
	errText := stderr.String()
	for _, want := range []string{"marker not found: section", "<!-- section:start -->", "--insert-if-missing"} {
		if !strings.Contains(errText, want) {
			t.Fatalf("stderr = %q, want to contain %q", errText, want)
		}
	}
}

func TestBodyReadMarkerMissingDoesNotSuggestInsertIfMissing(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "plain body"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "read", "123", "--marker", "section"}, strings.NewReader(""), &stdout, &stderr, gh)

	if code != ExitMarkerNotFound {
		t.Fatalf("exit code = %d, want %d", code, ExitMarkerNotFound)
	}
	errText := stderr.String()
	if !strings.Contains(errText, "marker not found: section") {
		t.Fatalf("stderr = %q, want marker not found", errText)
	}
	if strings.Contains(errText, "--insert-if-missing") {
		t.Fatalf("stderr = %q, read command should not suggest --insert-if-missing", errText)
	}
}

func TestBodyWriteInsertIfMissingRejectsPartialMarkerBlock(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "intro\n<!-- section:start -->\npartial\n"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "write", "123", "--marker", "section", "--insert-if-missing", "-"}, strings.NewReader("replacement\n"), &stdout, &stderr, gh)

	if code != ExitAmbiguousTarget {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitAmbiguousTarget, stderr.String())
	}
	if !strings.Contains(stderr.String(), "multiple or malformed marker blocks found in PR body: section") {
		t.Fatalf("stderr = %q, want ambiguous marker error", stderr.String())
	}
	if gh.updatedPRBody != "" {
		t.Fatalf("partial marker body was updated: %q", gh.updatedPRBody)
	}
}

func TestBodyWriteRejectsReplacementContainingSameMarkerToken(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "<!-- section:start -->\nold\n<!-- section:end -->"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "write", "123", "--marker", "section", "-"}, strings.NewReader("do not include <!-- section:start --> here\n"), &stdout, &stderr, gh)

	if code != ExitValidationError {
		t.Fatalf("exit code = %d, want %d", code, ExitValidationError)
	}
	errText := stderr.String()
	for _, want := range []string{"replacement contains marker token: section", "Disallowed tokens: <!-- section:start --> and <!-- section:end -->"} {
		if !strings.Contains(errText, want) {
			t.Fatalf("stderr = %q, want to contain %q", errText, want)
		}
	}
	if gh.updatedPRBody != "" {
		t.Fatalf("replacement marker token updated PR body: %q", gh.updatedPRBody)
	}
}

func TestBodyWriteMissingMarkerValueDoesNotTreatNextFlagAsMarker(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "plain body"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "write", "123", "--marker", "--file", "-"}, strings.NewReader("section\n"), &stdout, &stderr, gh)

	if code != ExitValidationError {
		t.Fatalf("exit code = %d, want %d", code, ExitValidationError)
	}
	if !strings.Contains(stderr.String(), "missing value for --marker") {
		t.Fatalf("stderr = %q, want missing marker value", stderr.String())
	}
	if gh.updatedPRBody != "" {
		t.Fatalf("invalid marker input updated PR body: %q", gh.updatedPRBody)
	}
}

func TestBodyWriteRejectsFileAndStdinTogether(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, fail: errors.New("unexpected api call")}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "write", "123", "--marker", "section", "-", "--file", "section.md"}, strings.NewReader("section\n"), &stdout, &stderr, gh)

	if code != ExitValidationError {
		t.Fatalf("exit code = %d, want %d", code, ExitValidationError)
	}
	if !strings.Contains(stderr.String(), "input specified twice") {
		t.Fatalf("stderr = %q, want duplicate input error", stderr.String())
	}
	if gh.updatedPRBody != "" {
		t.Fatalf("duplicate input updated PR body: %q", gh.updatedPRBody)
	}
}

func TestBodyWriteJSONReportsStructuredResult(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "<!-- section:start -->\nold\n<!-- section:end -->", URL: "https://example.test/pr/123"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "write", "123", "--marker", "section", "-", "--json"}, strings.NewReader("new\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	var result commandResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v; stdout=%s", err, stdout.String())
	}
	if result.Target != "pull_request_body" || result.PullNumber != 123 || result.Marker != "section" || !result.Updated || !result.Changed {
		t.Fatalf("result = %+v, want updated pull_request_body for PR 123 marker section", result)
	}
}

func TestBodyReadRangeJSONReportsBodySHA(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "one\ntwo\nthree\nfour\n", URL: "https://example.test/pr/123"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "read", "123", "--range", "2:3", "--json"}, strings.NewReader(""), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	var result struct {
		Target     string `json:"target"`
		PullNumber int    `json:"pull_number"`
		Range      string `json:"range"`
		BodySHA    string `json:"body_sha"`
		Body       string `json:"body"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v; stdout=%s", err, stdout.String())
	}
	if result.Target != "pull_request_body" || result.PullNumber != 123 || result.Range != "2:3" || result.Body != "two\nthree" {
		t.Fatalf("result = %+v, want selected PR body lines", result)
	}
	if result.BodySHA != twoThreeSHA {
		t.Fatalf("body_sha = %q, want current line-range sha", result.BodySHA)
	}
}

func TestBodyLinesDryRunAllowsUnguardedPreview(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "one\ntwo\nthree\nfour\n", URL: "https://example.test/pr/123"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "lines", "123", "--range", "2:3", "-", "--dry-run"}, strings.NewReader("new two\nnew three\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	if gh.updatedPRBody != "" {
		t.Fatalf("dry-run updated PR body: %q", gh.updatedPRBody)
	}
	out := stdout.String()
	for _, want := range []string{"body lines 2:3", " - |    1 | two", " - |    2 | three", " + |    1 | new two", " + |    2 | new three"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout = %q, want to contain %q", out, want)
		}
	}
}

func TestBodyLinesRequiresGuardForRealUpdate(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "one\ntwo\nthree\nfour\n"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "lines", "123", "--range", "2:3", "-"}, strings.NewReader("replacement\n"), &stdout, &stderr, gh)

	if code != ExitValidationError {
		t.Fatalf("exit code = %d, want %d", code, ExitValidationError)
	}
	if !strings.Contains(stderr.String(), "missing update guard") {
		t.Fatalf("stderr = %q, want missing update guard", stderr.String())
	}
	if gh.updatedPRBody != "" {
		t.Fatalf("unguarded line update changed PR body: %q", gh.updatedPRBody)
	}
}

func TestBodyLinesUpdatesRangeWhenExpectSHAMatches(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "one\ntwo\nthree\nfour\n", URL: "https://example.test/pr/123"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "lines", "123", "--range", "2:3", "--expect-sha", twoThreeSHA, "-"}, strings.NewReader("new two\nnew three\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	want := "one\nnew two\nnew three\nfour\n"
	if gh.updatedPRBody != want {
		t.Fatalf("updated PR body = %q, want %q", gh.updatedPRBody, want)
	}
}

func TestBodyLinesRejectsStaleExpectSHA(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "one\ntwo changed\nthree\nfour\n"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "lines", "123", "--range", "2:3", "--expect-sha", twoThreeSHA, "-"}, strings.NewReader("new\n"), &stdout, &stderr, gh)

	if code != ExitValidationError {
		t.Fatalf("exit code = %d, want %d", code, ExitValidationError)
	}
	if !strings.Contains(stderr.String(), "--expect-sha does not match") {
		t.Fatalf("stderr = %q, want stale sha error", stderr.String())
	}
	if gh.updatedPRBody != "" {
		t.Fatalf("stale sha updated PR body: %q", gh.updatedPRBody)
	}
}

func TestBodyLinesValidatesExpectFile(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "one\ntwo\nthree\n"}}
	expectFile := t.TempDir() + "/old.md"
	if err := os.WriteFile(expectFile, []byte("two\n"), 0o600); err != nil {
		t.Fatalf("write expect file: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "lines", "123", "--range", "2:2", "--expect-file", expectFile, "-"}, strings.NewReader("new two\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	if gh.updatedPRBody != "one\nnew two\nthree\n" {
		t.Fatalf("updated PR body = %q, want selected line replaced", gh.updatedPRBody)
	}
}

func TestBodyLinesRejectsStaleExpectFile(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "one\ntwo changed\nthree\n"}}
	expectFile := t.TempDir() + "/old.md"
	if err := os.WriteFile(expectFile, []byte("two\n"), 0o600); err != nil {
		t.Fatalf("write expect file: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "lines", "123", "--range", "2:2", "--expect-file", expectFile, "-"}, strings.NewReader("new two\n"), &stdout, &stderr, gh)

	if code != ExitValidationError {
		t.Fatalf("exit code = %d, want %d", code, ExitValidationError)
	}
	if !strings.Contains(stderr.String(), "--expect-file does not match") {
		t.Fatalf("stderr = %q, want stale expect-file error", stderr.String())
	}
	if gh.updatedPRBody != "" {
		t.Fatalf("stale expect-file updated PR body: %q", gh.updatedPRBody)
	}
}

func TestCommentUpsertCreatesMarkerWrappedCommentWhenMissing(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "upsert", "123", "--marker", "section", "-"}, strings.NewReader("section\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	want := "<!-- section:start -->\nsection\n<!-- section:end -->"
	if gh.createdComment != want {
		t.Fatalf("created comment = %q, want %q", gh.createdComment, want)
	}
}

func TestCommentUpsertUpdatesOnlyCurrentUserMarkerComment(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, currentLogin: "section-bot", comments: []Comment{
		{ID: 111, Author: "external-user", Body: "<!-- section:start -->\nattacker\n<!-- section:end -->", URL: "https://example.test/comment/111"},
		{ID: 222, Author: "section-bot", Body: "<!-- section:start -->\nold\n<!-- section:end -->", URL: "https://example.test/comment/222"},
	}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "upsert", "123", "--marker", "section", "-"}, strings.NewReader("new\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	if gh.updatedComments[222] != "<!-- section:start -->\nnew\n<!-- section:end -->" {
		t.Fatalf("bot comment update = %q, want marker-wrapped new content", gh.updatedComments[222])
	}
	if len(gh.updatedComments) != 1 {
		t.Fatalf("updated comments = %#v, want only current user's comment updated", gh.updatedComments)
	}
	if _, ok := gh.updatedComments[111]; ok {
		t.Fatalf("external user's comment was updated: %#v", gh.updatedComments)
	}
	if gh.createdComment != "" {
		t.Fatalf("upsert created comment instead of updating current user's comment: %q", gh.createdComment)
	}
}

func TestCommentUpsertDryRunDoesNotCreateMissingComment(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "upsert", "123", "--marker", "section", "-", "--dry-run"}, strings.NewReader("section\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	if gh.createdComment != "" {
		t.Fatalf("dry-run created comment: %q", gh.createdComment)
	}
	out := stdout.String()
	for _, want := range []string{"<!-- section:start -->", "op | line | content", " + |    1 | section", "<!-- section:end -->"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout = %q, want to contain %q", out, want)
		}
	}
}

func TestCommentUpsertRejectsReplacementContainingSameMarkerToken(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "upsert", "123", "--marker", "section", "-"}, strings.NewReader("bad <!-- section:end --> content\n"), &stdout, &stderr, gh)

	if code != ExitValidationError {
		t.Fatalf("exit code = %d, want %d", code, ExitValidationError)
	}
	if !strings.Contains(stderr.String(), "replacement contains marker token: section") {
		t.Fatalf("stderr = %q, want marker token validation", stderr.String())
	}
	if !strings.Contains(stderr.String(), "gh-prx comment upsert 123 --marker section --file section.md") {
		t.Fatalf("stderr = %q, want comment upsert retry", stderr.String())
	}
	if gh.createdComment != "" {
		t.Fatalf("replacement marker token created comment: %q", gh.createdComment)
	}
}

func TestCommentWriteRejectsCommentIDOutsidePullRequest(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, comments: []Comment{
		{ID: 111, Body: "comment in PR 123", URL: "https://example.test/comment/111"},
	}, commentsByID: map[int64]Comment{
		222: {ID: 222, Body: "comment in another PR", URL: "https://example.test/comment/222"},
	}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "write", "123", "--comment-id", "222", "--marker", "section", "-"}, strings.NewReader("section\n"), &stdout, &stderr, gh)

	if code != ExitAmbiguousTarget {
		t.Fatalf("exit code = %d, want %d", code, ExitAmbiguousTarget)
	}
	if !strings.Contains(stderr.String(), "comment not found in pull request: 222") {
		t.Fatalf("stderr = %q, want comment mismatch error", stderr.String())
	}
	if gh.updatedComments != nil {
		t.Fatalf("mismatched comment id updated comments: %#v", gh.updatedComments)
	}
}

func TestCommentWriteRequiresMarkerUnlessWhole(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, fail: errors.New("unexpected api call")}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "write", "123", "--comment-id", "111", "-"}, strings.NewReader("replacement\n"), &stdout, &stderr, gh)

	if code != ExitValidationError {
		t.Fatalf("exit code = %d, want %d", code, ExitValidationError)
	}
	errText := stderr.String()
	for _, want := range []string{"missing required flag: --marker", "--whole", "gh-prx comment write 123 --comment-id 123456 --marker section --file section.md"} {
		if !strings.Contains(errText, want) {
			t.Fatalf("stderr = %q, want to contain %q", errText, want)
		}
	}
	if gh.updatedComments != nil {
		t.Fatalf("markerless write updated comments: %#v", gh.updatedComments)
	}
}

func TestCommentWriteWholeRequiresExplicitFlag(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, comments: []Comment{
		{ID: 111, Body: "<!-- section:start -->\nold\n<!-- section:end -->", URL: "https://example.test/comment/111"},
	}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "write", "123", "--comment-id", "111", "--whole", "-"}, strings.NewReader("replacement\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	if gh.updatedComments[111] != "replacement" {
		t.Fatalf("updated comment = %q, want whole replacement", gh.updatedComments[111])
	}
	if !strings.Contains(stdout.String(), " - |") || !strings.Contains(stdout.String(), " + |    1 | replacement") {
		t.Fatalf("stdout = %q, want whole-comment diff", stdout.String())
	}
}

func TestCommentWriteRejectsMarkerAndWholeTogether(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, fail: errors.New("unexpected api call")}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "write", "123", "--comment-id", "111", "--marker", "section", "--whole", "-"}, strings.NewReader("replacement\n"), &stdout, &stderr, gh)

	if code != ExitValidationError {
		t.Fatalf("exit code = %d, want %d", code, ExitValidationError)
	}
	if !strings.Contains(stderr.String(), "conflicting flags: --marker and --whole") {
		t.Fatalf("stderr = %q, want conflicting flag error", stderr.String())
	}
	if gh.updatedComments != nil {
		t.Fatalf("conflicting flags updated comments: %#v", gh.updatedComments)
	}
}

func TestCommentWriteRejectsWholeAndInsertIfMissingTogether(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, fail: errors.New("unexpected api call")}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "write", "123", "--comment-id", "111", "--whole", "--insert-if-missing", "-"}, strings.NewReader("replacement\n"), &stdout, &stderr, gh)

	if code != ExitValidationError {
		t.Fatalf("exit code = %d, want %d", code, ExitValidationError)
	}
	if !strings.Contains(stderr.String(), "conflicting flags: --whole and --insert-if-missing") {
		t.Fatalf("stderr = %q, want conflicting flag error", stderr.String())
	}
	if gh.updatedComments != nil {
		t.Fatalf("conflicting flags updated comments: %#v", gh.updatedComments)
	}
}

func TestCommentReadRejectsCommentIDOutsidePullRequest(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, comments: []Comment{
		{ID: 111, Body: "comment in PR 123", URL: "https://example.test/comment/111"},
	}, commentsByID: map[int64]Comment{
		222: {ID: 222, Body: "comment in another PR", URL: "https://example.test/comment/222"},
	}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "read", "123", "--comment-id", "222"}, strings.NewReader(""), &stdout, &stderr, gh)

	if code != ExitAmbiguousTarget {
		t.Fatalf("exit code = %d, want %d", code, ExitAmbiguousTarget)
	}
	if !strings.Contains(stderr.String(), "comment not found in pull request: 222") {
		t.Fatalf("stderr = %q, want comment mismatch error", stderr.String())
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty output", stdout.String())
	}
}

func TestCommentReadRejectsNegativeCommentIDAsValidationError(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "read", "123", "--comment-id", "-1"}, strings.NewReader(""), &stdout, &stderr, gh)

	if code != ExitValidationError {
		t.Fatalf("exit code = %d, want %d", code, ExitValidationError)
	}
	if !strings.Contains(stderr.String(), "invalid flag: --comment-id") {
		t.Fatalf("stderr = %q, want invalid comment id error", stderr.String())
	}
}

func TestCommentWriteRejectsNegativeCommentIDAsValidationError(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, fail: errors.New("unexpected api call")}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "write", "123", "--comment-id", "-1", "--marker", "section", "-"}, strings.NewReader("section\n"), &stdout, &stderr, gh)

	if code != ExitValidationError {
		t.Fatalf("exit code = %d, want %d", code, ExitValidationError)
	}
	if !strings.Contains(stderr.String(), "invalid required flag: --comment-id") {
		t.Fatalf("stderr = %q, want invalid required comment id error", stderr.String())
	}
	if gh.updatedComments != nil {
		t.Fatalf("negative comment id updated comments: %#v", gh.updatedComments)
	}
}

func TestCommentUpsertRejectsAmbiguousMarkerMatches(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, comments: []Comment{
		{ID: 111, Body: "<!-- section:start -->\na\n<!-- section:end -->", Author: "github-actions[bot]", UpdatedAt: "2026-06-06T10:20:00Z"},
		{ID: 222, Body: "<!-- section:start -->\nb\n<!-- section:end -->", Author: "github-actions[bot]", UpdatedAt: "2026-06-06T10:25:00Z"},
	}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "upsert", "123", "--marker", "section", "-"}, strings.NewReader("section\n"), &stdout, &stderr, gh)

	if code != ExitAmbiguousTarget {
		t.Fatalf("exit code = %d, want %d", code, ExitAmbiguousTarget)
	}
	errText := stderr.String()
	for _, want := range []string{"multiple comments matched marker: section", "comment_id=111", "comment_id=222"} {
		if !strings.Contains(errText, want) {
			t.Fatalf("stderr = %q, want to contain %q", errText, want)
		}
	}
	if gh.createdComment != "" {
		t.Fatalf("ambiguous upsert created comment: %q", gh.createdComment)
	}
	if gh.updatedComments != nil {
		t.Fatalf("ambiguous upsert updated comments: %#v", gh.updatedComments)
	}
}

func TestCommentUpsertRejectsDuplicateMarkerBlocksInsideMatchedComment(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, comments: []Comment{
		{ID: 111, Body: "<!-- section:start -->\na\n<!-- section:end -->\n<!-- section:start -->\nb\n<!-- section:end -->", Author: "github-actions[bot]", UpdatedAt: "2026-06-06T10:20:00Z"},
	}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "upsert", "123", "--marker", "section", "-"}, strings.NewReader("replacement\n"), &stdout, &stderr, gh)

	if code != ExitAmbiguousTarget {
		t.Fatalf("exit code = %d, want %d", code, ExitAmbiguousTarget)
	}
	errText := stderr.String()
	for _, want := range []string{"multiple or malformed marker blocks found in comment: section", "comment_id=111"} {
		if !strings.Contains(errText, want) {
			t.Fatalf("stderr = %q, want to contain %q", errText, want)
		}
	}
	if gh.createdComment != "" {
		t.Fatalf("duplicate marker upsert created comment: %q", gh.createdComment)
	}
	if gh.updatedComments != nil {
		t.Fatalf("duplicate marker upsert updated comments: %#v", gh.updatedComments)
	}
}
