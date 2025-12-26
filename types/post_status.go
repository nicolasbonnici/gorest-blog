package types

type PostStatus string

const (
	PostStatusDrafted   PostStatus = "drafted"
	PostStatusPublished PostStatus = "published"
)

func (s PostStatus) String() string {
	return string(s)
}

func (s PostStatus) IsValid() bool {
	switch s {
	case PostStatusDrafted, PostStatusPublished:
		return true
	default:
		return false
	}
}
