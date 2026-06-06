package prx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

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
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "<!-- summary:start -->\nsummary\n<!-- summary:end -->"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "read", "123", "--marker", "summary", "--plain"}, strings.NewReader(""), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	if stdout.String() != "summary\n" {
		t.Fatalf("stdout = %q, want summary newline", stdout.String())
	}
}

func TestBodyWriteDryRunPrintsDiffWithoutUpdating(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "<!-- summary:start -->\nold summary\n<!-- summary:end -->", URL: "https://example.test/pr/123"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "write", "123", "--marker", "summary", "-", "--dry-run"}, strings.NewReader("new summary\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	if gh.updatedPRBody != "" {
		t.Fatalf("dry-run updated PR body: %q", gh.updatedPRBody)
	}
	out := stdout.String()
	for _, want := range []string{"<!-- summary:start -->", "op | line | content", " - |    1 | old summary", " + |    1 | new summary", "<!-- summary:end -->"} {
		if !strings.Contains(out, want) {
			t.Fatalf("diff output = %q, want to contain %q", out, want)
		}
	}
}

func TestBodyWriteNoChangesReturnsDedicatedExitCode(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "<!-- summary:start -->\nsame\n<!-- summary:end -->"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "write", "123", "--marker", "summary", "-"}, strings.NewReader("same\n"), &stdout, &stderr, gh)

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

	code := Run([]string{"body", "write", "123", "--marker", "summary", "-"}, strings.NewReader("summary\n"), &stdout, &stderr, gh)

	if code != ExitMarkerNotFound {
		t.Fatalf("exit code = %d, want %d", code, ExitMarkerNotFound)
	}
	errText := stderr.String()
	for _, want := range []string{"marker not found: summary", "<!-- summary:start -->", "--insert-if-missing"} {
		if !strings.Contains(errText, want) {
			t.Fatalf("stderr = %q, want to contain %q", errText, want)
		}
	}
}

func TestBodyWriteMissingMarkerValueDoesNotTreatNextFlagAsMarker(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "plain body"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "write", "123", "--marker", "--file", "-"}, strings.NewReader("summary\n"), &stdout, &stderr, gh)

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

	code := Run([]string{"body", "write", "123", "--marker", "summary", "-", "--file", "summary.md"}, strings.NewReader("summary\n"), &stdout, &stderr, gh)

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
	gh := &fakeGitHub{expectedPRNumber: 123, pr: PullRequest{Number: 123, Body: "<!-- summary:start -->\nold\n<!-- summary:end -->", URL: "https://example.test/pr/123"}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"body", "write", "123", "--marker", "summary", "-", "--json"}, strings.NewReader("new\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	var result commandResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v; stdout=%s", err, stdout.String())
	}
	if result.Target != "pull_request_body" || result.PullNumber != 123 || result.Marker != "summary" || !result.Updated || !result.Changed {
		t.Fatalf("result = %+v, want updated pull_request_body for PR 123 marker summary", result)
	}
}

func TestCommentUpsertCreatesMarkerWrappedCommentWhenMissing(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "upsert", "123", "--marker", "review", "-"}, strings.NewReader("review\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	want := "<!-- review:start -->\nreview\n<!-- review:end -->"
	if gh.createdComment != want {
		t.Fatalf("created comment = %q, want %q", gh.createdComment, want)
	}
}

func TestCommentUpsertUpdatesOnlyCurrentUserMarkerComment(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, currentLogin: "review-bot", comments: []Comment{
		{ID: 111, Author: "external-user", Body: "<!-- review:start -->\nattacker\n<!-- review:end -->", URL: "https://example.test/comment/111"},
		{ID: 222, Author: "review-bot", Body: "<!-- review:start -->\nold\n<!-- review:end -->", URL: "https://example.test/comment/222"},
	}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "upsert", "123", "--marker", "review", "-"}, strings.NewReader("new\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	if gh.updatedComments[222] != "<!-- review:start -->\nnew\n<!-- review:end -->" {
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

	code := Run([]string{"comment", "upsert", "123", "--marker", "review", "-", "--dry-run"}, strings.NewReader("review\n"), &stdout, &stderr, gh)

	if code != ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitSuccess, stderr.String())
	}
	if gh.createdComment != "" {
		t.Fatalf("dry-run created comment: %q", gh.createdComment)
	}
	out := stdout.String()
	for _, want := range []string{"<!-- review:start -->", "op | line | content", " + |    1 | review", "<!-- review:end -->"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout = %q, want to contain %q", out, want)
		}
	}
}

func TestCommentWriteRejectsCommentIDOutsidePullRequest(t *testing.T) {
	gh := &fakeGitHub{expectedPRNumber: 123, comments: []Comment{
		{ID: 111, Body: "comment in PR 123", URL: "https://example.test/comment/111"},
	}, commentsByID: map[int64]Comment{
		222: {ID: 222, Body: "comment in another PR", URL: "https://example.test/comment/222"},
	}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "write", "123", "--comment-id", "222", "-"}, strings.NewReader("review\n"), &stdout, &stderr, gh)

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

	code := Run([]string{"comment", "write", "123", "--comment-id", "-1", "-"}, strings.NewReader("review\n"), &stdout, &stderr, gh)

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
		{ID: 111, Body: "<!-- review:start -->\na\n<!-- review:end -->", Author: "github-actions[bot]", UpdatedAt: "2026-06-06T10:20:00Z"},
		{ID: 222, Body: "<!-- review:start -->\nb\n<!-- review:end -->", Author: "github-actions[bot]", UpdatedAt: "2026-06-06T10:25:00Z"},
	}}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"comment", "upsert", "123", "--marker", "review", "-"}, strings.NewReader("review\n"), &stdout, &stderr, gh)

	if code != ExitAmbiguousTarget {
		t.Fatalf("exit code = %d, want %d", code, ExitAmbiguousTarget)
	}
	errText := stderr.String()
	for _, want := range []string{"multiple comments matched marker: review", "comment_id=111", "comment_id=222"} {
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
