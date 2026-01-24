package importer

import (
	"fmt"

	"github.com/nicolasbonnici/gorest-blog/importer/engines"
)

type Post = engines.Post

type ImportOptions struct {
	Source         string
	UserID         string
	UpdateExisting bool
	DryRun         bool
	Username       string
	ArticleURL     string
	ArticleID      string
}

type ImportResult struct {
	TotalFetched int
	Created      int
	Updated      int
	Skipped      int
	Failed       int
	Errors       []error
}

func (r *ImportResult) Success() int {
	return r.Created + r.Updated
}

func (r *ImportResult) String() string {
	return fmt.Sprintf(
		"Import completed: %d fetched, %d created, %d updated, %d skipped, %d failed",
		r.TotalFetched, r.Created, r.Updated, r.Skipped, r.Failed,
	)
}

type ProgressReporter interface {
	Start(total int, message string)
	Update(current int, message string)
	Finish(message string)
	Error(err error)
}
