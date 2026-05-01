package importer

import (
	"fmt"

	"github.com/nicolasbonnici/gorest-blog/importer/engines"
)

type Post = engines.Post

type SyncMode string

const (
	SyncModeLocalWins  SyncMode = "local-wins"
	SyncModeRemoteWins SyncMode = "remote-wins"
	SyncModeImportOnly SyncMode = "import-only"
)

func (m SyncMode) IsValid() bool {
	switch m {
	case SyncModeLocalWins, SyncModeRemoteWins, SyncModeImportOnly:
		return true
	default:
		return false
	}
}

type ImportOptions struct {
	Source         string
	UserID         string
	DryRun         bool
	Truncate       bool
	Username       string
	ArticleURL     string
	ArticleID      string
	ImportComments bool
	SyncMode       SyncMode
	APIKey         string
}

type ImportResult struct {
	TotalFetched    int
	Created         int
	Updated         int
	Skipped         int
	Failed          int
	CommentsCreated int
	Errors          []error
}

func (r *ImportResult) Success() int {
	return r.Created + r.Updated
}

func (r *ImportResult) String() string {
	if r.CommentsCreated > 0 {
		return fmt.Sprintf(
			"Import completed: %d fetched, %d created, %d updated, %d skipped, %d failed, %d comments imported",
			r.TotalFetched, r.Created, r.Updated, r.Skipped, r.Failed, r.CommentsCreated,
		)
	}
	return fmt.Sprintf(
		"Import completed: %d fetched, %d created, %d updated, %d skipped, %d failed",
		r.TotalFetched, r.Created, r.Updated, r.Skipped, r.Failed,
	)
}

type SyncResult struct {
	LocalCreated  int
	LocalUpdated  int
	RemoteCreated int
	RemoteUpdated int
	Skipped       int
	Errors        []SyncError
}

type SyncError struct {
	PostSlug  string
	Operation string
	Error     error
}

func (r *SyncResult) Success() int {
	return r.LocalCreated + r.LocalUpdated + r.RemoteCreated + r.RemoteUpdated
}

func (r *SyncResult) String() string {
	return fmt.Sprintf(
		"Sync completed: %d local created, %d local updated, %d remote created, %d remote updated, %d skipped, %d errors",
		r.LocalCreated, r.LocalUpdated, r.RemoteCreated, r.RemoteUpdated, r.Skipped, len(r.Errors),
	)
}

type ProgressReporter interface {
	Start(total int, message string)
	Update(current int, message string)
	Finish(message string)
	Error(err error)
}
