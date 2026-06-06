package prx

const (
	ExitSuccess         = 0
	ExitValidationError = 1
	ExitMarkerNotFound  = 2
	ExitAmbiguousTarget = 3
	ExitGitHubAPIError  = 4
	ExitNoChanges       = 5
)

type PullRequest struct {
	Number int
	Body   string
	URL    string
}

type Comment struct {
	ID        int64
	Body      string
	URL       string
	Author    string
	UpdatedAt string
}

type GitHubClient interface {
	GetPullRequest(number int) (PullRequest, error)
	UpdatePullRequestBody(number int, body string) (PullRequest, error)
	ListComments(number int) ([]Comment, error)
	GetComment(id int64) (Comment, error)
	UpdateComment(id int64, body string) (Comment, error)
	CreateComment(number int, body string) (Comment, error)
}
