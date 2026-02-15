package engines

type Post struct {
	ID            string
	Title         string
	Content       string
	Slug          string
	PublishedAt   string
	UpdatedAt     string
	URL           string
	SourceID      string
	LikesCount    int
	CommentsCount int
	ViewsCount    int
	Comments      []Comment
}

type Comment struct {
	ID        string
	Content   string
	CreatedAt string
	ParentID  string
	Author    CommentAuthor
	Children  []Comment
}

type CommentAuthor struct {
	Name     string
	Username string
}
