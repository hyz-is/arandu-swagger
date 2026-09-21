package ui

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

// PageOptions configures the rendered HTML shell for Swagger UI.
type PageOptions struct {
	Title              string
	UIPath             string
	Favicon            string
	Locale             string
	HasTheme           bool
	Theme              PageThemeOptions
	HasCustomJS        bool
	HTMXBoost          bool
	BackURL            string
	BackText           string
	BackTarget         string
	DisableThemeToggle bool
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
	DefaultDark          bool
	DisableThemeToggle   bool
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

	hasLogo := opts.Theme.Logo != nil && opts.Theme.Logo.URL != ""
	hasBack := opts.BackURL != ""
	hasToggle := !opts.DisableThemeToggle

	if hasLogo || hasBack || hasToggle {
		page.WriteString("<header class=\"arandu-swagger-topbar\">\n")
		page.WriteString("  <div class=\"arandu-swagger-topbar-wrapper\">\n")
		page.WriteString("    <div class=\"arandu-swagger-topbar-start\">\n")
		if hasLogo {
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
			page.WriteString("      <a href=\"")
			page.WriteString(html.EscapeString(href))
			page.WriteString("\" class=\"arandu-swagger-logo-link\" target=\"")
			page.WriteString(html.EscapeString(target))
			page.WriteString("\">\n")
			page.WriteString("        <img src=\"")
			page.WriteString(html.EscapeString(opts.Theme.Logo.URL))
			page.WriteString("\" alt=\"")
			page.WriteString(html.EscapeString(alt))
			page.WriteString("\" class=\"arandu-swagger-logo\">\n")
			if alt != "" {
				page.WriteString("        <span class=\"arandu-swagger-title\">")
				page.WriteString(html.EscapeString(alt))
				page.WriteString("</span>\n")
			}
			page.WriteString("      </a>\n")
		}
		page.WriteString("    </div>\n")
		page.WriteString("    <div class=\"arandu-swagger-topbar-end\">\n")
		if hasBack {
			backText := opts.BackText
			if backText == "" {
				if strings.HasPrefix(strings.ToLower(locale), "pt") {
					backText = "Voltar para o site"
				} else {
					backText = "Back to site"
				}
			}
			backTarget := opts.BackTarget
			if backTarget == "" {
				backTarget = "_self"
			}
			page.WriteString("      <a href=\"")
			page.WriteString(html.EscapeString(opts.BackURL))
			page.WriteString("\" class=\"arandu-swagger-back-link\" target=\"")
			page.WriteString(html.EscapeString(backTarget))
			page.WriteString("\">\n")
			page.WriteString("        <svg class=\"arandu-swagger-icon\" viewBox=\"0 0 256 256\" width=\"16\" height=\"16\" fill=\"currentColor\" aria-hidden=\"true\"><path d=\"M224,128a8,8,0,0,1-8,8H59.31l58.35,58.34a8,8,0,0,1-11.32,11.32l-72-72a8,8,0,0,1,0-11.32l72-72a8,8,0,0,1,11.32,11.32L59.31,120H216A8,8,0,0,1,224,128Z\"/></svg>\n")
			page.WriteString("        <span>")
			page.WriteString(html.EscapeString(backText))
			page.WriteString("</span>\n")
			page.WriteString("      </a>\n")
		}
		if hasToggle {
			toggleLabel := "Alternar tema"
			if !strings.HasPrefix(strings.ToLower(locale), "pt") {
				toggleLabel = "Toggle theme"
			}
			page.WriteString("      <button type=\"button\" class=\"arandu-swagger-theme-toggle\" aria-label=\"")
			page.WriteString(html.EscapeString(toggleLabel))
			page.WriteString("\" title=\"")
			page.WriteString(html.EscapeString(toggleLabel))
			page.WriteString("\">\n")
			page.WriteString("        <span class=\"arandu-swagger-glyph-light\" aria-hidden=\"true\"><svg viewBox=\"0 0 256 256\" width=\"18\" height=\"18\" fill=\"currentColor\"><path d=\"M120,40V16a8,8,0,0,1,16,0V40a8,8,0,0,1-16,0Zm72,88a64,64,0,1,1-64-64A64.07,64.07,0,0,1,192,128Zm-16,0a48,48,0,1,0-48,48A48.05,48.05,0,0,0,176,128ZM58.34,69.66A8,8,0,0,0,69.66,58.34l-16-16A8,8,0,0,0,42.34,53.66Zm0,116.68-16,16a8,8,0,0,0,11.32,11.32l16-16a8,8,0,0,0-11.32-11.32ZM192,72a8,8,0,0,0,5.66-2.34l16-16a8,8,0,0,0-11.32-11.32l-16,16A8,8,0,0,0,192,72Zm5.66,114.34a8,8,0,0,0-11.32,11.32l16,16a8,8,0,0,0,11.32-11.32ZM48,128a8,8,0,0,0-8-8H16a8,8,0,0,0,0,16H40A8,8,0,0,0,48,128Zm80,80a8,8,0,0,0-8,8v24a8,8,0,0,0,16,0V216A8,8,0,0,0,128,208Zm112-88H216a8,8,0,0,0,0,16h24a8,8,0,0,0,0-16Z\"/></svg></span>\n")
			page.WriteString("        <span class=\"arandu-swagger-glyph-dark\" aria-hidden=\"true\"><svg viewBox=\"0 0 256 256\" width=\"18\" height=\"18\" fill=\"currentColor\"><path d=\"M233.54,142.23a8,8,0,0,0-8-2,88.08,88.08,0,0,1-109.8-109.8,8,8,0,0,0-10-10,104.84,104.84,0,0,0-52.91,37A104,104,0,0,0,136,224a103.09,103.09,0,0,0,62.52-20.88,104.84,104.84,0,0,0,37-52.91A8,8,0,0,0,233.54,142.23ZM188.9,190.34A88,88,0,0,1,65.66,67.11a89,89,0,0,1,31.4-26A106,106,0,0,0,96,56,104.11,104.11,0,0,0,200,160a106,106,0,0,0,14.92-1.06A89,89,0,0,1,188.9,190.34Z\"/></svg></span>\n")
			page.WriteString("      </button>\n")
		}
		page.WriteString("    </div>\n")
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
	if !opts.DisableThemeToggle {
		script.WriteString(fmt.Sprintf(`    var defaultDark = %t;
    function applyTheme(theme) {
      if (theme === "light") {
        document.documentElement.setAttribute("data-theme", "light");
        document.documentElement.classList.remove("dark");
        if (document.body) {
          document.body.classList.remove("dark-theme");
          document.body.classList.add("light-theme");
        }
      } else {
        document.documentElement.setAttribute("data-theme", "dark");
        document.documentElement.classList.add("dark");
        if (document.body) {
          document.body.classList.remove("light-theme");
          document.body.classList.add("dark-theme");
        }
      }
    }
    var savedTheme = null;
    try {
      savedTheme = localStorage.getItem("arandu-swagger-theme");
    } catch (e) {}
    if (!savedTheme) {
      savedTheme = defaultDark ? "dark" : (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
    }
    applyTheme(savedTheme);
    var btn = document.querySelector(".arandu-swagger-theme-toggle");
    if (btn && !btn.getAttribute("data-theme-bound")) {
      btn.setAttribute("data-theme-bound", "true");
      btn.addEventListener("click", function() {
        var current = document.documentElement.getAttribute("data-theme") === "light" ? "light" : "dark";
        var next = current === "light" ? "dark" : "light";
        try {
          localStorage.setItem("arandu-swagger-theme", next);
        } catch (e) {}
        applyTheme(next);
      });
    }
`, opts.DefaultDark))
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
