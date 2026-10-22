package app

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gofiber/fiber/v2"
)

// openAPIDoc caches the OpenAPI definition loaded from api/openapi.yaml at
// startup. Missing file fails fast, matching the service's boot philosophy.
var openAPIDoc []byte

// openAPIPath resolves api/openapi.yaml relative to the module root (found by
// walking up from this file until go.mod appears) so the file is located
// whether the binary runs from the repo root or tests run from the package
// directory, both of which set a different working directory.
func openAPIPath() string {
	_, file, _, ok := runtime.Caller(1) // caller is init in docs.go
	path := "api/openapi.yaml"
	if ok {
		dir := filepath.Dir(file)
		for {
			if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
				path = filepath.Join(dir, "api", "openapi.yaml")
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return path
}

func init() {
	var err error
	openAPIDoc, err = os.ReadFile(openAPIPath())
	if err != nil {
		panic("failed to load api/openapi.yaml: " + err.Error())
	}
}

// openAPIHandler serves the machine-readable OpenAPI 3.0 definition.
func openAPIHandler(c *fiber.Ctx) error {
	c.Set("Content-Type", "application/vnd.oai.openapi+yaml; charset=utf-8")
	return c.Status(http.StatusOK).Send(openAPIDoc)
}

// docsHTML embeds Scalar's reference UI from CDN, pointing at /openapi.yaml.
const docsHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Payment Gateway API Reference</title>
</head>
<body>
  <div id="scalar"></div>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  <script>
    window.addEventListener('DOMContentLoaded', () => {
      Scalar.APIReference({
        url: '/openapi.yaml',
        container: document.getElementById('scalar')
      });
    });
  </script>
</body>
</html>`

// docsHandler serves Scalar's API reference UI.
func docsHandler(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(http.StatusOK).SendString(docsHTML)
}
