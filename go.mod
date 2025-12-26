module github.com/nicolasbonnici/gorest-blog-plugin

go 1.25.1

require (
	github.com/gofiber/fiber/v2 v2.52.10
	github.com/nicolasbonnici/gorest v0.4.0
	github.com/schollz/progressbar/v3 v3.14.1
	golang.org/x/crypto v0.46.0
)

// Temporary: Use feat/migrations branch until GoREST 0.4 is released
// Remove this once GoREST 0.4.0 is published to main
replace github.com/nicolasbonnici/gorest => github.com/nicolasbonnici/gorest feat-migrations
