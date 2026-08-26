package ui

import (
	"encoding/json"
	"html"
	"strings"
)

// Page returns the HTML shell for the embedded Swagger UI distribution.
// Every executable resource is external so applications can serve the page
// with a content security policy that does not allow inline code.
func Page(title, uiPath string) []byte {
	stylesheet, _ := Path("swagger-ui.css")
	bundle, _ := Path("swagger-ui-bundle.js")

	var page strings.Builder
	page.WriteString("<!doctype html>\n")
	page.WriteString("<html lang=\"en\">\n<head>\n")
	page.WriteString("<meta charset=\"utf-8\">\n")
	page.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	page.WriteString("<title>")
	page.WriteString(html.EscapeString(title))
	page.WriteString("</title>\n")
	page.WriteString("<link rel=\"stylesheet\" href=\"")
	page.WriteString(html.EscapeString(uiPath + "/" + stylesheet))
	page.WriteString("\">\n")
	page.WriteString("</head>\n<body>\n")
	page.WriteString("<div id=\"swagger-ui\"></div>\n")
	page.WriteString("<script src=\"")
	page.WriteString(html.EscapeString(uiPath + "/" + bundle))
	page.WriteString("\"></script>\n")
	page.WriteString("<script src=\"")
	page.WriteString(html.EscapeString(uiPath + "/swagger-initializer.js"))
	page.WriteString("\"></script>\n")
	page.WriteString("</body>\n</html>\n")
	return []byte(page.String())
}

// Initializer returns the external JavaScript that configures Swagger UI for
// the same-origin specification endpoint.
func Initializer(specPath string, persistAuthorization, disableTryItOut bool) []byte {
	encodedPath, _ := json.Marshal(specPath)

	var script strings.Builder
	script.WriteString("window.addEventListener(\"load\", function () {\n")
	script.WriteString("  window.ui = SwaggerUIBundle({\n")
	script.WriteString("    url: ")
	script.Write(encodedPath)
	script.WriteString(",\n")
	script.WriteString("    dom_id: \"#swagger-ui\",\n")
	script.WriteString("    deepLinking: true,\n")
	script.WriteString("    validatorUrl: null,\n")
	script.WriteString("    layout: \"BaseLayout\",\n")
	script.WriteString("    presets: [SwaggerUIBundle.presets.apis],\n")
	if persistAuthorization {
		script.WriteString("    persistAuthorization: true,\n")
	}
	if disableTryItOut {
		script.WriteString("    supportedSubmitMethods: [],\n")
	}
	script.WriteString("  });\n")
	script.WriteString("});\n")
	return []byte(script.String())
}
