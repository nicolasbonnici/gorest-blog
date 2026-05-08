package devto

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/nicolasbonnici/gorest-blog/importer/engines"
)

func MapPost(devtoArticle DevToArticle, comments []DevToComment) engines.Post {
	// Only set PublishedAt if it's not a zero time
	publishedAt := ""
	if !devtoArticle.PublishedAt.IsZero() {
		publishedAt = devtoArticle.PublishedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	updatedAt := ""
	if !devtoArticle.EditedAt.IsZero() {
		updatedAt = devtoArticle.EditedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	// Unescape HTML entities in markdown content (Dev.to encodes quotes, etc.)
	content := html.UnescapeString(devtoArticle.BodyMarkdown)

	return engines.Post{
		ID:            fmt.Sprintf("%d", devtoArticle.ID),
		Title:         devtoArticle.Title,
		Content:       content,
		Slug:          devtoArticle.Slug,
		PublishedAt:   publishedAt,
		UpdatedAt:     updatedAt,
		URL:           devtoArticle.URL,
		SourceID:      fmt.Sprintf("devto-%d", devtoArticle.ID),
		LikesCount:    devtoArticle.PublicReactions,
		CommentsCount: devtoArticle.CommentsCount,
		ViewsCount:    0, // dev.to doesn't expose view counts via API
		Comments:      MapComments(comments, ""),
	}
}

func MapComments(devtoComments []DevToComment, parentID string) []engines.Comment {
	comments := make([]engines.Comment, 0, len(devtoComments))
	for _, dc := range devtoComments {
		comments = append(comments, MapComment(dc, parentID))
	}
	return comments
}

func MapComment(devtoComment DevToComment, parentID string) engines.Comment {
	createdAt := ""
	if !devtoComment.CreatedAt.IsZero() {
		createdAt = devtoComment.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	// Convert HTML content to plain text (basic conversion)
	content := htmlToMarkdown(devtoComment.BodyHTML)

	comment := engines.Comment{
		ID:        devtoComment.ID,
		Content:   content,
		CreatedAt: createdAt,
		ParentID:  parentID,
		Author: engines.CommentAuthor{
			Name:     devtoComment.User.Name,
			Username: devtoComment.User.Username,
		},
	}

	// Recursively map children comments
	if len(devtoComment.Children) > 0 {
		comment.Children = MapComments(devtoComment.Children, comment.ID)
	}

	return comment
}

// Basic HTML to Markdown conversion that preserves code blocks
func htmlToMarkdown(htmlContent string) string {
	content := htmlContent

	// Preserve code blocks first - convert <pre><code> to markdown code blocks
	// Handle multi-line code blocks with language hint
	preCodeRe := regexp.MustCompile(`(?s)<pre><code[^>]*class="[^"]*language-([^"]+)"[^>]*>(.*?)</code></pre>`)
	content = preCodeRe.ReplaceAllStringFunc(content, func(match string) string {
		submatches := preCodeRe.FindStringSubmatch(match)
		if len(submatches) == 3 {
			lang := submatches[1]
			code := html.UnescapeString(submatches[2])
			return "\n```" + lang + "\n" + code + "\n```\n"
		}
		return match
	})

	// Handle code blocks without language hint
	preCodeSimpleRe := regexp.MustCompile(`(?s)<pre><code>(.*?)</code></pre>`)
	content = preCodeSimpleRe.ReplaceAllStringFunc(content, func(match string) string {
		submatches := preCodeSimpleRe.FindStringSubmatch(match)
		if len(submatches) == 2 {
			code := html.UnescapeString(submatches[1])
			return "\n```\n" + code + "\n```\n"
		}
		return match
	})

	// Handle inline code blocks
	inlineCodeRe := regexp.MustCompile(`<code>(.*?)</code>`)
	content = inlineCodeRe.ReplaceAllStringFunc(content, func(match string) string {
		submatches := inlineCodeRe.FindStringSubmatch(match)
		if len(submatches) == 2 {
			code := html.UnescapeString(submatches[1])
			return "`" + code + "`"
		}
		return match
	})

	// Convert common HTML elements to markdown
	content = strings.ReplaceAll(content, "<br>", "\n")
	content = strings.ReplaceAll(content, "<br/>", "\n")
	content = strings.ReplaceAll(content, "<br />", "\n")

	// Convert paragraphs
	content = regexp.MustCompile(`<p[^>]*>`).ReplaceAllString(content, "\n\n")
	content = strings.ReplaceAll(content, "</p>", "")

	// Convert strong/bold
	strongRe := regexp.MustCompile(`<(?:strong|b)>(.*?)</(?:strong|b)>`)
	content = strongRe.ReplaceAllString(content, "**$1**")

	// Convert emphasis/italic
	emRe := regexp.MustCompile(`<(?:em|i)>(.*?)</(?:em|i)>`)
	content = emRe.ReplaceAllString(content, "*$1*")

	// Convert links
	linkRe := regexp.MustCompile(`<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	content = linkRe.ReplaceAllString(content, "[$2]($1)")

	// Now remove any remaining HTML tags
	content = stripHTMLTags(content)

	// Unescape HTML entities
	content = html.UnescapeString(content)

	// Clean up excessive whitespace while preserving intentional line breaks
	content = regexp.MustCompile(`\n{3,}`).ReplaceAllString(content, "\n\n")
	content = strings.TrimSpace(content)

	return content
}

func stripHTMLTags(content string) string {
	// Remove all remaining HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(content, "")
}
