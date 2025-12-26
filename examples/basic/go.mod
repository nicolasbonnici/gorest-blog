module myapp

go 1.25.1

require (
	github.com/nicolasbonnici/gorest v0.4.0
	github.com/nicolasbonnici/gorest-blog-plugin v1.0.0
)

replace github.com/nicolasbonnici/gorest => github.com/nicolasbonnici/gorest v0.0.0-feat-migrations
replace github.com/nicolasbonnici/gorest-blog-plugin => ../..
