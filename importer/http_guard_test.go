package importer

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// TestRegisterRoutes_GuardsRunBeforeHandler ensures the guards supplied to
// RegisterRoutes execute ahead of the import handler, so an unauthorized caller
// is rejected before any import logic (or nil db dereference) is reached.
func TestRegisterRoutes_GuardsRunBeforeHandler(t *testing.T) {
	app := fiber.New()

	denied := false
	denyGuard := func(c fiber.Ctx) error {
		denied = true
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	// db is nil on purpose: if the handler ran, it would panic or error rather
	// than return 401, proving the guard did not gate the route.
	RegisterRoutes(app, nil, denyGuard)

	for _, path := range []string{"/api/import/devto", "/api/import/engines"} {
		method := "POST"
		if path == "/api/import/engines" {
			method = "GET"
		}
		req := httptest.NewRequest(method, path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s %s: unexpected error: %v", method, path, err)
		}
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("%s %s: want 401 from guard, got %d", method, path, resp.StatusCode)
		}
	}

	if !denied {
		t.Fatal("guard was never invoked")
	}
}

// TestRegisterRoutes_NoGuardsReachesHandler verifies that without guards the
// routes are mounted bare (the handler is reached). The engines handler is used
// because it needs no database.
func TestRegisterRoutes_NoGuardsReachesHandler(t *testing.T) {
	app := fiber.New()
	RegisterRoutes(app, nil)

	req := httptest.NewRequest("GET", "/api/import/engines", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("want 200 reaching handler, got %d", resp.StatusCode)
	}
}
