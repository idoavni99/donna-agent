package api

import (
	"embed"
	"net/http"
)

//go:embed openapi.yaml
var apiDocs embed.FS

func registerDocs(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.yaml", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		content, err := apiDocs.ReadFile("openapi.yaml")
		if err != nil {
			http.Error(writer, "OpenAPI specification unavailable", http.StatusInternalServerError)
			return
		}
		_, _ = writer.Write(content)
	})
	mux.HandleFunc("GET /swagger", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(swaggerHTML))
	})
}

const swaggerHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Donna API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => SwaggerUIBundle({ url: "/openapi.yaml", dom_id: "#swagger-ui" });
  </script>
</body>
</html>`
