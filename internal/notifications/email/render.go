package email

import (
	"embed"
	htmltemplate "html/template"
	"strings"
)

//go:embed templates/*.html templates/*.txt
var emailTemplates embed.FS

// Render parses and executes the named template with the given data.
func render(name string, data any) (string, error) {
	path := "templates/" + name

	tmpl, err := htmltemplate.New(name).ParseFS(emailTemplates, path)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
