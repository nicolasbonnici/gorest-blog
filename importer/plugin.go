package importer

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest/plugin"

	_ "github.com/nicolasbonnici/gorest-blog/importer/engines/devto"
)

type Plugin struct{}

func NewPlugin() plugin.Plugin {
	return &Plugin{}
}

func (p *Plugin) Name() string {
	return "importer"
}

func (p *Plugin) Initialize(config map[string]interface{}) error {
	return nil
}

func (p *Plugin) Handler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Next()
	}
}
