package prx

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type commandResult struct {
	Target     string   `json:"target"`
	PullNumber int      `json:"pull_number,omitempty"`
	CommentID  int64    `json:"comment_id,omitempty"`
	Marker     string   `json:"marker,omitempty"`
	Updated    bool     `json:"updated"`
	Changed    bool     `json:"changed"`
	DryRun     bool     `json:"dry_run"`
	URL        string   `json:"url,omitempty"`
	Message    string   `json:"message,omitempty"`
	ExitCode   int      `json:"exit_code,omitempty"`
	Error      string   `json:"error,omitempty"`
	ErrorKind  string   `json:"error_kind,omitempty"`
	Fix        []string `json:"fix,omitempty"`
	Retry      string   `json:"retry,omitempty"`
}

type appError struct {
	Code    int
	Kind    string
	Message string
	Fix     []string
	Retry   string
}

func (e appError) Error() string {
	return e.Message
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, gh GitHubClient) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, rootHelp())
		return ExitSuccess
	}

	code, err := dispatch(args, stdin, stdout, gh)
	if err == nil {
		return code
	}
	jsonOutput := hasFlag(args, "--json")
	if jsonOutput {
		result := commandResult{Error: err.Error(), ErrorKind: errorKind(err), ExitCode: errorCode(err)}
		var app appError
		if errors.As(err, &app) {
			result.Fix = app.Fix
			result.Retry = app.Retry
		}
		writeJSON(stdout, result)
	} else {
		writeError(stderr, err)
	}
	return errorCode(err)
}

func dispatch(args []string, stdin io.Reader, stdout io.Writer, gh GitHubClient) (int, error) {
	switch args[0] {
	case "body":
		return runBody(args[1:], stdin, stdout, gh)
	case "comment":
		return runComment(args[1:], stdin, stdout, gh)
	default:
		return ExitValidationError, appError{
			Code:    ExitValidationError,
			Kind:    "validation_error",
			Message: "unknown command: " + args[0],
			Fix:     []string{"Use one of: body, comment."},
			Retry:   "gh-prx --help",
		}
	}
}

func runBody(args []string, stdin io.Reader, stdout io.Writer, gh GitHubClient) (int, error) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, bodyHelp())
		return ExitSuccess, nil
	}
	switch args[0] {
	case "read":
		return runBodyRead(args[1:], stdout, gh)
	case "write", "patch":
		return runBodyWrite(args[1:], stdin, stdout, gh)
	default:
		return ExitValidationError, validationError("unknown body command: "+args[0], "Use: gh-prx body read ... or gh-prx body write ...", "gh-prx body --help")
	}
}

func runBodyRead(args []string, stdout io.Writer, gh GitHubClient) (int, error) {
	fs := flag.NewFlagSet("body read", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	marker := fs.String("marker", "", "HTML comment marker name")
	plain := fs.Bool("plain", false, "print only marker content")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlagSet(fs, args); err != nil {
		return ExitValidationError, validationError(err.Error(), "Use a PR number and optional --marker.", "gh-prx body read 123 --marker ai-summary")
	}
	prNumber, err := onePRNumber(fs.Args(), "body read")
	if err != nil {
		return ExitValidationError, err
	}
	pr, err := gh.GetPullRequest(prNumber)
	if err != nil {
		return ExitGitHubAPIError, apiError(err)
	}
	output := pr.Body
	if *marker != "" {
		output, err = readMarker(pr.Body, *marker, *plain)
		if err != nil {
			return ExitMarkerNotFound, markerError("body", prNumber, *marker, "")
		}
	}
	if *jsonOut {
		writeJSON(stdout, map[string]any{
			"target":      "pull_request_body",
			"pull_number": prNumber,
			"marker":      *marker,
			"body":        output,
			"url":         pr.URL,
		})
		return ExitSuccess, nil
	}
	fmt.Fprint(stdout, output)
	if output != "" && !strings.HasSuffix(output, "\n") {
		fmt.Fprintln(stdout)
	}
	return ExitSuccess, nil
}

func runBodyWrite(args []string, stdin io.Reader, stdout io.Writer, gh GitHubClient) (int, error) {
	fs := flag.NewFlagSet("body write", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	marker := fs.String("marker", "", "HTML comment marker name")
	file := fs.String("file", "", "read replacement content from file")
	insert := fs.Bool("insert-if-missing", false, "append marker block if it does not exist")
	dryRun := fs.Bool("dry-run", false, "print diff without updating GitHub")
	jsonOut := fs.Bool("json", false, "print JSON")
	yes := fs.Bool("yes", false, "run non-interactively")
	if err := parseFlagSet(fs, args); err != nil {
		return ExitValidationError, validationError(err.Error(), "Use a PR number, --marker, and --file or -.", "gh-prx body write 123 --marker ai-summary --file summary.md")
	}
	_ = yes
	prNumber, inputPath, err := writeTarget(fs.Args(), "body write")
	if err != nil {
		return ExitValidationError, err
	}
	if *marker == "" {
		return ExitValidationError, validationError("missing required flag: --marker", "Choose the named marker block to update.", "gh-prx body write 123 --marker ai-summary --file summary.md")
	}
	replacement, err := readInput(stdin, inputPath, *file)
	if err != nil {
		return ExitValidationError, err
	}
	pr, err := gh.GetPullRequest(prNumber)
	if err != nil {
		return ExitGitHubAPIError, apiError(err)
	}
	newBody, oldContent, newContent, err := replaceMarker(pr.Body, *marker, replacement)
	if err != nil {
		if !*insert {
			return ExitMarkerNotFound, markerError("body", prNumber, *marker, "gh-prx body write "+strconv.Itoa(prNumber)+" --marker "+*marker+" --file summary.md --insert-if-missing")
		}
		oldContent = ""
		newContent = strings.TrimSuffix(replacement, "\n")
		newBody = insertMarkerIfMissing(pr.Body, *marker, replacement)
	}
	diff := renderMarkerDiff(*marker, oldContent, newContent)
	if newBody == pr.Body {
		return writeNoChanges(stdout, *jsonOut, commandResult{Target: "pull_request_body", PullNumber: prNumber, Marker: *marker, URL: pr.URL})
	}
	if *dryRun {
		return writeSuccess(stdout, *jsonOut, commandResult{Target: "pull_request_body", PullNumber: prNumber, Marker: *marker, Updated: false, Changed: true, DryRun: true, URL: pr.URL, Message: diff}, diff)
	}
	updated, err := gh.UpdatePullRequestBody(prNumber, newBody)
	if err != nil {
		return ExitGitHubAPIError, apiError(err)
	}
	return writeSuccess(stdout, *jsonOut, commandResult{Target: "pull_request_body", PullNumber: prNumber, Marker: *marker, Updated: true, Changed: true, URL: updated.URL, Message: diff}, diff)
}

func runComment(args []string, stdin io.Reader, stdout io.Writer, gh GitHubClient) (int, error) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, commentHelp())
		return ExitSuccess, nil
	}
	switch args[0] {
	case "read":
		return runCommentRead(args[1:], stdout, gh)
	case "write", "patch":
		return runCommentWrite(args[1:], stdin, stdout, gh)
	case "upsert":
		return runCommentUpsert(args[1:], stdin, stdout, gh)
	default:
		return ExitValidationError, validationError("unknown comment command: "+args[0], "Use: gh-prx comment read, write, or upsert.", "gh-prx comment --help")
	}
}

func runCommentRead(args []string, stdout io.Writer, gh GitHubClient) (int, error) {
	fs := flag.NewFlagSet("comment read", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	marker := fs.String("marker", "", "HTML comment marker name")
	commentID := fs.Int64("comment-id", 0, "specific issue comment id")
	plain := fs.Bool("plain", false, "print only marker content")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlagSet(fs, args); err != nil {
		return ExitValidationError, validationError(err.Error(), "Use a PR number plus --marker or --comment-id.", "gh-prx comment read 123 --marker ai-review")
	}
	prNumber, err := onePRNumber(fs.Args(), "comment read")
	if err != nil {
		return ExitValidationError, err
	}
	comment, err := selectComment(gh, prNumber, *marker, *commentID)
	if err != nil {
		return errorCode(err), err
	}
	output := comment.Body
	if *marker != "" {
		output, err = readMarker(comment.Body, *marker, *plain)
		if err != nil {
			return ExitMarkerNotFound, markerError("comment", prNumber, *marker, "")
		}
	}
	if *jsonOut {
		writeJSON(stdout, map[string]any{
			"target":      "pull_request_comment",
			"pull_number": prNumber,
			"comment_id":  comment.ID,
			"marker":      *marker,
			"body":        output,
			"url":         comment.URL,
		})
		return ExitSuccess, nil
	}
	fmt.Fprint(stdout, output)
	if output != "" && !strings.HasSuffix(output, "\n") {
		fmt.Fprintln(stdout)
	}
	return ExitSuccess, nil
}

func runCommentWrite(args []string, stdin io.Reader, stdout io.Writer, gh GitHubClient) (int, error) {
	fs := flag.NewFlagSet("comment write", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	marker := fs.String("marker", "", "HTML comment marker name")
	commentID := fs.Int64("comment-id", 0, "specific issue comment id")
	file := fs.String("file", "", "read replacement content from file")
	insert := fs.Bool("insert-if-missing", false, "append marker block if it does not exist")
	dryRun := fs.Bool("dry-run", false, "print diff without updating GitHub")
	jsonOut := fs.Bool("json", false, "print JSON")
	yes := fs.Bool("yes", false, "run non-interactively")
	if err := parseFlagSet(fs, args); err != nil {
		return ExitValidationError, validationError(err.Error(), "Use a PR number, --comment-id, and --file or -.", "gh-prx comment write 123 --comment-id 123456 --file review.md")
	}
	_ = yes
	prNumber, inputPath, err := writeTarget(fs.Args(), "comment write")
	if err != nil {
		return ExitValidationError, err
	}
	if *commentID <= 0 {
		return ExitValidationError, validationError("invalid required flag: --comment-id", "Choose one exact positive comment id to update.", "gh-prx comment write 123 --comment-id 123456 --file review.md")
	}
	replacement, err := readInput(stdin, inputPath, *file)
	if err != nil {
		return ExitValidationError, err
	}
	comment, err := commentByIDInPR(gh, prNumber, *commentID)
	if err != nil {
		return errorCode(err), err
	}
	newBody := strings.TrimSuffix(replacement, "\n")
	oldContent := comment.Body
	newContent := newBody
	diff := ""
	if *marker != "" {
		newBody, oldContent, newContent, err = replaceMarker(comment.Body, *marker, replacement)
		if err != nil {
			if !*insert {
				return ExitMarkerNotFound, markerError("comment", prNumber, *marker, "gh-prx comment write "+strconv.Itoa(prNumber)+" --comment-id "+strconv.FormatInt(*commentID, 10)+" --marker "+*marker+" --file review.md --insert-if-missing")
			}
			oldContent = ""
			newContent = strings.TrimSuffix(replacement, "\n")
			newBody = insertMarkerIfMissing(comment.Body, *marker, replacement)
		}
		diff = renderMarkerDiff(*marker, oldContent, newContent)
	} else {
		diff = renderWholeDiff(comment.Body, newBody)
	}
	if newBody == comment.Body {
		return writeNoChanges(stdout, *jsonOut, commandResult{Target: "pull_request_comment", PullNumber: prNumber, CommentID: *commentID, Marker: *marker, URL: comment.URL})
	}
	if *dryRun {
		return writeSuccess(stdout, *jsonOut, commandResult{Target: "pull_request_comment", PullNumber: prNumber, CommentID: *commentID, Marker: *marker, Updated: false, Changed: true, DryRun: true, URL: comment.URL, Message: diff}, diff)
	}
	updated, err := gh.UpdateComment(*commentID, newBody)
	if err != nil {
		return ExitGitHubAPIError, apiError(err)
	}
	return writeSuccess(stdout, *jsonOut, commandResult{Target: "pull_request_comment", PullNumber: prNumber, CommentID: updated.ID, Marker: *marker, Updated: true, Changed: true, URL: updated.URL, Message: diff}, diff)
}

func runCommentUpsert(args []string, stdin io.Reader, stdout io.Writer, gh GitHubClient) (int, error) {
	fs := flag.NewFlagSet("comment upsert", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	marker := fs.String("marker", "", "HTML comment marker name")
	file := fs.String("file", "", "read replacement content from file")
	dryRun := fs.Bool("dry-run", false, "print diff without updating GitHub")
	jsonOut := fs.Bool("json", false, "print JSON")
	yes := fs.Bool("yes", false, "run non-interactively")
	if err := parseFlagSet(fs, args); err != nil {
		return ExitValidationError, validationError(err.Error(), "Use a PR number, --marker, and --file or -.", "gh-prx comment upsert 123 --marker ai-review --file review.md")
	}
	_ = yes
	prNumber, inputPath, err := writeTarget(fs.Args(), "comment upsert")
	if err != nil {
		return ExitValidationError, err
	}
	if *marker == "" {
		return ExitValidationError, validationError("missing required flag: --marker", "Upsert needs a marker to find or create the managed comment.", "gh-prx comment upsert 123 --marker ai-review --file review.md")
	}
	replacement, err := readInput(stdin, inputPath, *file)
	if err != nil {
		return ExitValidationError, err
	}
	matches, err := matchingComments(gh, prNumber, *marker)
	if err != nil {
		return errorCode(err), err
	}
	newContent := strings.TrimSuffix(replacement, "\n")
	if len(matches) == 0 {
		body := markerBlock(*marker, replacement)
		diff := renderMarkerDiff(*marker, "", newContent)
		if *dryRun {
			return writeSuccess(stdout, *jsonOut, commandResult{Target: "pull_request_comment", PullNumber: prNumber, Marker: *marker, Updated: false, Changed: true, DryRun: true, Message: diff}, diff)
		}
		created, err := gh.CreateComment(prNumber, body)
		if err != nil {
			return ExitGitHubAPIError, apiError(err)
		}
		return writeSuccess(stdout, *jsonOut, commandResult{Target: "pull_request_comment", PullNumber: prNumber, CommentID: created.ID, Marker: *marker, Updated: true, Changed: true, URL: created.URL, Message: diff}, diff)
	}
	comment := matches[0]
	newBody, oldContent, _, err := replaceMarker(comment.Body, *marker, replacement)
	if err != nil {
		return ExitMarkerNotFound, markerError("comment", prNumber, *marker, "")
	}
	diff := renderMarkerDiff(*marker, oldContent, newContent)
	if newBody == comment.Body {
		return writeNoChanges(stdout, *jsonOut, commandResult{Target: "pull_request_comment", PullNumber: prNumber, CommentID: comment.ID, Marker: *marker, URL: comment.URL})
	}
	if *dryRun {
		return writeSuccess(stdout, *jsonOut, commandResult{Target: "pull_request_comment", PullNumber: prNumber, CommentID: comment.ID, Marker: *marker, Updated: false, Changed: true, DryRun: true, URL: comment.URL, Message: diff}, diff)
	}
	updated, err := gh.UpdateComment(comment.ID, newBody)
	if err != nil {
		return ExitGitHubAPIError, apiError(err)
	}
	return writeSuccess(stdout, *jsonOut, commandResult{Target: "pull_request_comment", PullNumber: prNumber, CommentID: updated.ID, Marker: *marker, Updated: true, Changed: true, URL: updated.URL, Message: diff}, diff)
}

func selectComment(gh GitHubClient, prNumber int, marker string, commentID int64) (Comment, error) {
	if commentID < 0 {
		return Comment{}, validationError("invalid flag: --comment-id", "Use a positive comment id.", "gh-prx comment read 123 --comment-id 123456")
	}
	if commentID != 0 {
		return commentByIDInPR(gh, prNumber, commentID)
	}
	if marker == "" {
		return Comment{}, validationError("missing required flag: --marker or --comment-id", "Choose a marker search or one exact comment.", "gh-prx comment read 123 --marker ai-review")
	}
	matches, err := matchingComments(gh, prNumber, marker)
	if err != nil {
		return Comment{}, err
	}
	if len(matches) == 0 {
		return Comment{}, markerError("comment", prNumber, marker, "")
	}
	return matches[0], nil
}

func commentByIDInPR(gh GitHubClient, prNumber int, commentID int64) (Comment, error) {
	comments, err := gh.ListComments(prNumber)
	if err != nil {
		return Comment{}, apiError(err)
	}
	for _, comment := range comments {
		if comment.ID == commentID {
			return comment, nil
		}
	}
	return Comment{}, appError{
		Code:    ExitAmbiguousTarget,
		Kind:    "ambiguous_target",
		Message: "comment not found in pull request: " + strconv.FormatInt(commentID, 10),
		Fix:     []string{"Confirm the comment belongs to the specified PR.", "Use gh-prx comment read <pr-number> --marker <marker> to list the managed target."},
		Retry:   "gh-prx comment read " + strconv.Itoa(prNumber) + " --marker ai-review",
	}
}

func matchingComments(gh GitHubClient, prNumber int, marker string) ([]Comment, error) {
	comments, err := gh.ListComments(prNumber)
	if err != nil {
		return nil, apiError(err)
	}
	matches := make([]Comment, 0)
	for _, comment := range comments {
		if containsMarker(comment.Body, marker) {
			matches = append(matches, comment)
		}
	}
	if len(matches) > 1 {
		return nil, ambiguousError(prNumber, marker, matches)
	}
	return matches, nil
}

func onePRNumber(args []string, command string) (int, error) {
	if len(args) != 1 {
		return 0, validationError("expected exactly one PR number for "+command, "Pass the pull request number as the first positional argument.", "gh-prx "+command+" 123 --marker ai-summary")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n <= 0 {
		return 0, validationError("invalid PR number: "+args[0], "Use a positive numeric pull request number.", "gh-prx "+command+" 123 --marker ai-summary")
	}
	return n, nil
}

func writeTarget(args []string, command string) (int, string, error) {
	if len(args) < 1 || len(args) > 2 {
		return 0, "", validationError("expected PR number and optional input path for "+command, "Use --file path or '-' for stdin.", "gh-prx "+command+" 123 --marker ai-summary --file summary.md")
	}
	prNumber, err := strconv.Atoi(args[0])
	if err != nil || prNumber <= 0 {
		return 0, "", validationError("invalid PR number: "+args[0], "Use a positive numeric pull request number.", "gh-prx "+command+" 123 --marker ai-summary --file summary.md")
	}
	inputPath := ""
	if len(args) == 2 {
		inputPath = args[1]
	}
	return prNumber, inputPath, nil
}

func readInput(stdin io.Reader, positionalPath, fileFlag string) (string, error) {
	if positionalPath != "" && fileFlag != "" {
		return "", validationError("input specified twice", "Use either --file path or '-' for stdin, not both.", "gh-prx body write 123 --marker ai-summary --file summary.md")
	}
	if fileFlag == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", validationError("cannot read stdin", "Pipe content into the command.", "cat summary.md | gh-prx body write 123 --marker ai-summary --file -")
		}
		return string(data), nil
	}
	if fileFlag != "" {
		data, err := os.ReadFile(fileFlag)
		if err != nil {
			return "", validationError("cannot read file: "+fileFlag, "Check that the file exists and is readable.", "gh-prx body write 123 --marker ai-summary --file "+fileFlag)
		}
		return string(data), nil
	}
	if positionalPath == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", validationError("cannot read stdin", "Pipe content into the command.", "cat summary.md | gh-prx body write 123 --marker ai-summary -")
		}
		return string(data), nil
	}
	if positionalPath != "" {
		data, err := os.ReadFile(positionalPath)
		if err != nil {
			return "", validationError("cannot read file: "+positionalPath, "Check that the file exists and is readable.", "gh-prx body write 123 --marker ai-summary --file "+positionalPath)
		}
		return string(data), nil
	}
	return "", validationError("missing input", "Pass replacement content with --file path or '-'.", "gh-prx body write 123 --marker ai-summary --file summary.md")
}

func writeSuccess(stdout io.Writer, jsonOut bool, result commandResult, text string) (int, error) {
	if jsonOut {
		writeJSON(stdout, result)
		return ExitSuccess, nil
	}
	fmt.Fprint(stdout, text)
	return ExitSuccess, nil
}

func writeNoChanges(stdout io.Writer, jsonOut bool, result commandResult) (int, error) {
	result.Changed = false
	result.Updated = false
	result.Message = "no changes"
	if jsonOut {
		writeJSON(stdout, result)
	} else {
		fmt.Fprintln(stdout, "no changes")
	}
	return ExitNoChanges, nil
}

func writeJSON(stdout io.Writer, value any) {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func writeError(stderr io.Writer, err error) {
	var app appError
	if !errors.As(err, &app) {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return
	}
	fmt.Fprintf(stderr, "error: %s\n\n", app.Message)
	if len(app.Fix) > 0 {
		fmt.Fprintln(stderr, "fix:")
		for i, fix := range app.Fix {
			fmt.Fprintf(stderr, "  %d. %s\n", i+1, fix)
		}
		fmt.Fprintln(stderr)
	}
	if app.Retry != "" {
		fmt.Fprintln(stderr, "retry:")
		fmt.Fprintf(stderr, "  %s\n", app.Retry)
	}
}

func errorCode(err error) int {
	var app appError
	if errors.As(err, &app) {
		return app.Code
	}
	return ExitGitHubAPIError
}

func errorKind(err error) string {
	var app appError
	if errors.As(err, &app) {
		return app.Kind
	}
	return "github_api_error"
}

func validationError(message, fix, retry string) appError {
	return appError{Code: ExitValidationError, Kind: "validation_error", Message: message, Fix: []string{fix}, Retry: retry}
}

func markerError(target string, prNumber int, marker, retry string) appError {
	start, end := markerTokens(marker)
	fix := []string{
		"Add the marker block to the PR " + target + ".",
		"Retry with --insert-if-missing when creating the block is intended.",
		"Expected markers: " + start + " and " + end + ".",
	}
	if retry == "" {
		retry = "gh-prx " + target + " read " + strconv.Itoa(prNumber) + " --marker " + marker
	}
	return appError{Code: ExitMarkerNotFound, Kind: "marker_not_found", Message: "marker not found: " + marker, Fix: fix, Retry: retry}
}

func ambiguousError(prNumber int, marker string, candidates []Comment) appError {
	lines := make([]string, 0, len(candidates)+1)
	lines = append(lines, "Pick one comment and rerun with --comment-id.")
	for _, c := range candidates {
		lines = append(lines, fmt.Sprintf("candidate comment_id=%d author=%s updated=%s", c.ID, c.Author, c.UpdatedAt))
	}
	retry := "gh-prx comment write " + strconv.Itoa(prNumber) + " --comment-id " + strconv.FormatInt(candidates[len(candidates)-1].ID, 10) + " --marker " + marker + " --file review.md"
	return appError{Code: ExitAmbiguousTarget, Kind: "ambiguous_target", Message: "multiple comments matched marker: " + marker, Fix: lines, Retry: retry}
}

func apiError(err error) appError {
	return appError{
		Code:    ExitGitHubAPIError,
		Kind:    "github_api_error",
		Message: err.Error(),
		Fix:     []string{"Check gh authentication, repository permissions, and network access."},
		Retry:   "gh auth status",
	}
}

func hasFlag(args []string, name string) bool {
	for _, arg := range args {
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}

func parseFlagSet(fs *flag.FlagSet, args []string) error {
	if err := validateValueFlags(args); err != nil {
		return err
	}
	return fs.Parse(normalizeFlags(args))
}

func validateValueFlags(args []string) error {
	valueFlags := map[string]bool{
		"--marker":     true,
		"--file":       true,
		"--comment-id": true,
	}
	for i, arg := range args {
		name := arg
		if idx := strings.Index(arg, "="); idx >= 0 {
			name = arg[:idx]
			if valueFlags[name] && arg[idx+1:] == "" {
				return fmt.Errorf("missing value for %s", name)
			}
		}
		if valueFlags[name] && !strings.Contains(arg, "=") {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return fmt.Errorf("missing value for %s", name)
			}
		}
	}
	return nil
}

func normalizeFlags(args []string) []string {
	valueFlags := map[string]bool{
		"--marker":     true,
		"--file":       true,
		"--comment-id": true,
	}
	var flags []string
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			flags = append(flags, arg)
			name := arg
			if idx := strings.Index(arg, "="); idx >= 0 {
				name = arg[:idx]
			}
			if valueFlags[name] && !strings.Contains(arg, "=") && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		positionals = append(positionals, arg)
	}
	return append(flags, positionals...)
}

func rootHelp() string {
	return `gh-prx treats GitHub pull request markdown fields as named read/write blocks.

Examples:
  gh-prx body read 123 --marker ai-summary
  gh-prx body read 123 --marker ai-summary --plain
  gh-prx body write 123 --marker ai-summary --file summary.md --dry-run
  cat summary.md | gh-prx body write 123 --marker ai-summary -
  gh-prx comment read 123 --marker ai-review
  gh-prx comment write 123 --comment-id 123456 --file review.md
  gh-prx comment upsert 123 --marker ai-review --file review.md

Exit codes:
  0  success
  1  validation error
  2  marker not found
  3  ambiguous target
  4  GitHub API error
  5  no changes

Use --json for machine-readable output and --dry-run to preview writes.
`
}

func bodyHelp() string {
	return `Examples:
  gh-prx body read 123 --marker ai-summary
  gh-prx body read 123 --marker ai-summary --plain
  gh-prx body write 123 --marker ai-summary --file summary.md
  gh-prx body write 123 --marker ai-summary --file summary.md --dry-run
  cat summary.md | gh-prx body write 123 --marker ai-summary -
`
}

func commentHelp() string {
	return `Examples:
  gh-prx comment read 123 --marker ai-review
  gh-prx comment read 123 --comment-id 123456 --plain
  gh-prx comment write 123 --comment-id 123456 --file review.md
  gh-prx comment write 123 --comment-id 123456 --marker ai-review --file review.md --dry-run
  gh-prx comment upsert 123 --marker ai-review --file review.md
`
}
