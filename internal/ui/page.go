package ui

import (
	"encoding/json"
	"html"
	"strings"
)

// PageOptions configures the rendered HTML shell for Swagger UI.
type PageOptions struct {
	Title       string
	UIPath      string
	Favicon     string
	Locale      string
	HasTheme    bool
	Theme       PageThemeOptions
	HasCustomJS bool
	HTMXBoost   bool
}

// PageThemeOptions holds theme-specific rendering settings for the HTML shell.
type PageThemeOptions struct {
	Logo *PageLogoOptions
}

// PageLogoOptions holds logo properties for rendering in the topbar.
type PageLogoOptions struct {
	URL    string
	Href   string
	Alt    string
	Target string
}

// InitializerOptions configures the JavaScript initializer for Swagger UI.
type InitializerOptions struct {
	SpecPath             string
	PersistAuthorization bool
	DisableTryItOut      bool
	HTMX                 bool
	Locale               string
	Translations         map[string]string
}

// Page returns the HTML shell for the embedded Swagger UI distribution.
// Every executable resource is external so applications can serve the page
// with a content security policy that does not allow inline code.
func Page(opts PageOptions) []byte {
	stylesheet, _ := Path("swagger-ui.css")
	bundle, _ := Path("swagger-ui-bundle.js")

	locale := opts.Locale
	if locale == "" {
		locale = "en"
	}

	var page strings.Builder
	page.WriteString("<!doctype html>\n")
	page.WriteString("<html lang=\"")
	page.WriteString(html.EscapeString(locale))
	page.WriteString("\">\n<head>\n")
	page.WriteString("<meta charset=\"utf-8\">\n")
	page.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	page.WriteString("<title>")
	page.WriteString(html.EscapeString(opts.Title))
	page.WriteString("</title>\n")

	if opts.Favicon != "" {
		page.WriteString("<link rel=\"icon\" href=\"")
		page.WriteString(html.EscapeString(opts.Favicon))
		page.WriteString("\">\n")
	}

	page.WriteString("<link rel=\"stylesheet\" href=\"")
	page.WriteString(html.EscapeString(opts.UIPath + "/" + stylesheet))
	page.WriteString("\">\n")

	if opts.HasTheme {
		page.WriteString("<link rel=\"stylesheet\" href=\"")
		page.WriteString(html.EscapeString(opts.UIPath + "/theme.css"))
		page.WriteString("\">\n")
	}

	page.WriteString("</head>\n<body>\n")

	if opts.Theme.Logo != nil && opts.Theme.Logo.URL != "" {
		page.WriteString("<header class=\"arandu-swagger-topbar\">\n")
		page.WriteString("  <div class=\"arandu-swagger-topbar-wrapper\">\n")
		href := opts.Theme.Logo.Href
		if href == "" {
			href = "/"
		}
		target := opts.Theme.Logo.Target
		if target == "" {
			target = "_self"
		}
		alt := opts.Theme.Logo.Alt
		if alt == "" {
			alt = opts.Title
		}
		page.WriteString("    <a href=\"")
		page.WriteString(html.EscapeString(href))
		page.WriteString("\" class=\"arandu-swagger-logo-link\" target=\"")
		page.WriteString(html.EscapeString(target))
		page.WriteString("\">\n")
		page.WriteString("      <img src=\"")
		page.WriteString(html.EscapeString(opts.Theme.Logo.URL))
		page.WriteString("\" alt=\"")
		page.WriteString(html.EscapeString(alt))
		page.WriteString("\" class=\"arandu-swagger-logo\">\n")
		if alt != "" {
			page.WriteString("      <span class=\"arandu-swagger-title\">")
			page.WriteString(html.EscapeString(alt))
			page.WriteString("</span>\n")
		}
		page.WriteString("    </a>\n")
		page.WriteString("  </div>\n")
		page.WriteString("</header>\n")
	}

	if opts.HTMXBoost {
		page.WriteString("<div id=\"swagger-ui\" hx-boost=\"false\"></div>\n")
	} else {
		page.WriteString("<div id=\"swagger-ui\"></div>\n")
	}

	page.WriteString("<script src=\"")
	page.WriteString(html.EscapeString(opts.UIPath + "/" + bundle))
	page.WriteString("\"></script>\n")
	page.WriteString("<script src=\"")
	page.WriteString(html.EscapeString(opts.UIPath + "/swagger-initializer.js"))
	page.WriteString("\"></script>\n")

	if opts.HasCustomJS {
		page.WriteString("<script src=\"")
		page.WriteString(html.EscapeString(opts.UIPath + "/theme.js"))
		page.WriteString("\"></script>\n")
	}

	page.WriteString("</body>\n</html>\n")
	return []byte(page.String())
}

// Initializer returns the external JavaScript that configures Swagger UI for
// the same-origin specification endpoint.
func Initializer(opts InitializerOptions) []byte {
	encodedPath, _ := json.Marshal(opts.SpecPath)

	var script strings.Builder
	script.WriteString("(function () {\n")
	script.WriteString("  function initSwagger() {\n")
	script.WriteString("    var container = document.getElementById(\"swagger-ui\");\n")
	script.WriteString("    if (!container) return;\n")
	script.WriteString("    if (container.getAttribute(\"data-swagger-initialized\") === \"true\" && container.children.length > 0) return;\n")
	script.WriteString("    container.setAttribute(\"data-swagger-initialized\", \"true\");\n\n")

	script.WriteString("    window.ui = SwaggerUIBundle({\n")
	script.WriteString("      url: ")
	script.Write(encodedPath)
	script.WriteString(",\n")
	script.WriteString("      dom_id: \"#swagger-ui\",\n")
	script.WriteString("      deepLinking: true,\n")
	script.WriteString("      validatorUrl: null,\n")
	script.WriteString("      layout: \"BaseLayout\",\n")
	script.WriteString("      presets: [SwaggerUIBundle.presets.apis],\n")
	if opts.PersistAuthorization {
		script.WriteString("      persistAuthorization: true,\n")
	}
	if opts.DisableTryItOut {
		script.WriteString("      supportedSubmitMethods: [],\n")
	}
	script.WriteString("    });\n")

	translations := ResolveTranslations(opts.Locale, opts.Translations)
	if len(translations) > 0 {
		encodedTranslations, _ := json.Marshal(translations)
		script.WriteString("\n    var translations = ")
		script.Write(encodedTranslations)
		script.WriteString(";\n")
		script.WriteString(`    function translateNode(root) {
      if (!root || !translations) return;
      var inputs = root.querySelectorAll ? root.querySelectorAll('input[placeholder], textarea[placeholder]') : [];
      for (var i = 0; i < inputs.length; i++) {
        var ph = inputs[i].getAttribute('placeholder');
        if (ph && translations[ph]) {
          inputs[i].setAttribute('placeholder', translations[ph]);
        }
      }
      var walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
        acceptNode: function(node) {
          if (!node || !node.nodeValue) return NodeFilter.FILTER_REJECT;
          var parent = node.parentElement;
          if (!parent) return NodeFilter.FILTER_REJECT;
          var tag = parent.tagName.toLowerCase();
          if (tag === 'script' || tag === 'style' || tag === 'code' || tag === 'pre') return NodeFilter.FILTER_REJECT;
          if (parent.closest && (parent.closest('.highlight-code') || parent.closest('.microlight') || parent.closest('.curl-command') || parent.closest('.swagger-ui-schema-preview'))) return NodeFilter.FILTER_REJECT;
          var val = node.nodeValue.trim();
          if (val && translations[val]) return NodeFilter.FILTER_ACCEPT;
          return NodeFilter.FILTER_SKIP;
        }
      });
      var nodes = [];
      var n;
      while ((n = walker.nextNode())) {
        nodes.push(n);
      }
      for (var j = 0; j < nodes.length; j++) {
        var tn = nodes[j];
        var text = tn.nodeValue.trim();
        if (translations[text]) {
          var lead = tn.nodeValue.match(/^\s*/)[0];
          var trail = tn.nodeValue.match(/\s*$/)[0];
          tn.nodeValue = lead + translations[text] + trail;
        }
      }
    }

    var isTranslating = false;
    var observer = new MutationObserver(function(mutations) {
      if (isTranslating) return;
      isTranslating = true;
      try {
        for (var i = 0; i < mutations.length; i++) {
          var m = mutations[i];
          if (m.type === 'childList') {
            for (var j = 0; j < m.addedNodes.length; j++) {
              var added = m.addedNodes[j];
              if (added.nodeType === Node.ELEMENT_NODE) {
                translateNode(added);
              } else if (added.nodeType === Node.TEXT_NODE) {
                var v = added.nodeValue ? added.nodeValue.trim() : '';
                if (v && translations[v]) {
                  var l = added.nodeValue.match(/^\s*/)[0];
                  var t = added.nodeValue.match(/\s*$/)[0];
                  added.nodeValue = l + translations[v] + t;
                }
              }
            }
          }
        }
      } finally {
        isTranslating = false;
      }
    });

    observer.observe(container, { childList: true, subtree: true });
    observer.observe(document.body, { childList: true, subtree: true });
    translateNode(container);
`)
	}
	script.WriteString("  }\n\n")

	script.WriteString("  if (document.readyState === \"complete\" || document.readyState === \"interactive\") {\n")
	script.WriteString("    initSwagger();\n")
	script.WriteString("  } else {\n")
	script.WriteString("    window.addEventListener(\"DOMContentLoaded\", initSwagger);\n")
	script.WriteString("  }\n")

	if opts.HTMX {
		script.WriteString("\n  // HTMX lifecycle integration\n")
		script.WriteString("  document.addEventListener(\"htmx:load\", function (evt) {\n")
		script.WriteString("    if (!evt.detail || !evt.detail.elt || evt.detail.elt.querySelector(\"#swagger-ui\") || evt.detail.elt.id === \"swagger-ui\") {\n")
		script.WriteString("      initSwagger();\n")
		script.WriteString("    }\n")
		script.WriteString("  });\n")
		script.WriteString("  document.addEventListener(\"htmx:afterSettle\", function (evt) {\n")
		script.WriteString("    if (!evt.detail || !evt.detail.elt || evt.detail.elt.querySelector(\"#swagger-ui\") || evt.detail.elt.id === \"swagger-ui\") {\n")
		script.WriteString("      initSwagger();\n")
		script.WriteString("    }\n")
		script.WriteString("  });\n")
	}

	script.WriteString("})();\n")
	return []byte(script.String())
}
