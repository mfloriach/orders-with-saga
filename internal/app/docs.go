package app

import (
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
)

// openAPIDoc caches the OpenAPI definition loaded from api/openapi.yaml at
// startup. Missing file fails fast, matching the service's boot philosophy.
var openAPIDoc []byte

func init() {
	var err error
	openAPIDoc, err = os.ReadFile("api/openapi.yaml")
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
