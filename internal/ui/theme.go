package ui

import (
	"fmt"
	"strings"
)

// ThemeOptions provides color, styling, and branding properties for CSS generation.
type ThemeOptions struct {
	DarkMode        bool
	PrimaryColor    string
	BackgroundColor string
	CardColor       string
	TextColor       string
	MutedColor      string
	BorderColor     string
	CustomCSS       string
}

// GenerateThemeCSS generates the complete CSS stylesheet for Swagger UI.
func GenerateThemeCSS(opts ThemeOptions) []byte {
	primary := opts.PrimaryColor
	bg := opts.BackgroundColor
	card := opts.CardColor
	text := opts.TextColor
	muted := opts.MutedColor
	border := opts.BorderColor

	if opts.DarkMode {
		if primary == "" {
			primary = "#14b8a6"
		}
		if bg == "" {
			bg = "#0a0a0a"
		}
		if card == "" {
			card = "#121214"
		}
		if text == "" {
			text = "#f4f4f5"
		}
		if muted == "" {
			muted = "#a1a1aa"
		}
		if border == "" {
			border = "#27272a"
		}
	} else {
		if primary == "" {
			primary = "#4990e2"
		}
		if bg == "" {
			bg = "#ffffff"
		}
		if card == "" {
			card = "#fafafa"
		}
		if text == "" {
			text = "#3b4151"
		}
		if muted == "" {
			muted = "#6b7280"
		}
		if border == "" {
			border = "#e5e7eb"
		}
	}

	var css strings.Builder

	css.WriteString(":root {\n")
	css.WriteString(fmt.Sprintf("  --swagger-primary: %s;\n", primary))
	css.WriteString(fmt.Sprintf("  --swagger-bg: %s;\n", bg))
	css.WriteString(fmt.Sprintf("  --swagger-card: %s;\n", card))
	css.WriteString(fmt.Sprintf("  --swagger-text: %s;\n", text))
	css.WriteString(fmt.Sprintf("  --swagger-muted: %s;\n", muted))
	css.WriteString(fmt.Sprintf("  --swagger-border: %s;\n", border))
	css.WriteString("}\n\n")

	css.WriteString(`html, body {
  margin: 0;
  padding: 0;
  background-color: var(--swagger-bg);
  color: var(--swagger-text);
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
}

.swagger-ui {
  color: var(--swagger-text);
}

.swagger-ui .topbar {
  display: none !important;
}

.arandu-swagger-topbar {
  background-color: var(--swagger-card);
  border-bottom: 1px solid var(--swagger-border);
  padding: 12px 24px;
  display: flex;
  align-items: center;
}

.arandu-swagger-topbar-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
  max-width: 1460px;
  width: 100%;
  margin: 0 auto;
}

.arandu-swagger-logo-link {
  display: flex;
  align-items: center;
  gap: 10px;
  text-decoration: none;
  color: var(--swagger-text);
  font-weight: 600;
  font-size: 16px;
  letter-spacing: -0.01em;
}

.arandu-swagger-logo {
  height: 28px;
  width: auto;
  max-width: 180px;
  object-fit: contain;
}

.arandu-swagger-title {
  color: var(--swagger-text);
}

.swagger-ui .info {
  margin: 32px 0 24px;
}

.swagger-ui .info .title {
  color: var(--swagger-text);
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.swagger-ui .info .title small {
  background-color: rgba(20, 184, 166, 0.15);
  color: var(--swagger-primary);
  border-radius: 6px;
  padding: 2px 8px;
  font-size: 13px;
  font-weight: 600;
  vertical-align: middle;
}

.swagger-ui .info .title small.version-stamp {
  background-color: var(--swagger-primary);
  color: #000000;
  font-weight: 700;
}

.swagger-ui .info p,
.swagger-ui .info li,
.swagger-ui .info table {
  color: var(--swagger-muted);
  font-size: 14px;
  line-height: 1.6;
}

.swagger-ui .info a {
  color: var(--swagger-primary);
  text-decoration: none;
}

.swagger-ui .info a:hover {
  text-decoration: underline;
}

.swagger-ui .scheme-container {
  background-color: var(--swagger-card);
  border: 1px solid var(--swagger-border);
  border-radius: 8px;
  box-shadow: none;
  padding: 16px 20px;
  margin: 24px 0;
}

.swagger-ui .schemes-title {
  color: var(--swagger-text);
  font-weight: 600;
}

.swagger-ui .schemes > label {
  color: var(--swagger-text);
}

.swagger-ui .btn {
  border-radius: 6px;
  font-weight: 500;
  transition: all 0.15s ease-in-out;
}

.swagger-ui .btn.authorize {
  background-color: rgba(20, 184, 166, 0.1) !important;
  color: var(--swagger-primary) !important;
  border: 1px solid var(--swagger-primary) !important;
  border-radius: 6px !important;
  padding: 6px 14px !important;
  font-size: 13px !important;
  display: inline-flex !important;
  align-items: center !important;
  gap: 8px !important;
  box-shadow: none !important;
}

.swagger-ui .btn.authorize:hover {
  background-color: var(--swagger-primary) !important;
  color: #000000 !important;
}

.swagger-ui .btn.authorize svg {
  fill: currentColor !important;
}

.swagger-ui .btn.execute {
  background-color: var(--swagger-primary) !important;
  border: 1px solid var(--swagger-primary) !important;
  color: #000000 !important;
  font-weight: 600 !important;
  border-radius: 6px !important;
  padding: 6px 16px !important;
}

.swagger-ui .btn.execute:hover {
  opacity: 0.9 !important;
}

.swagger-ui .btn.cancel {
  border: 1px solid var(--swagger-border) !important;
  background-color: transparent !important;
  color: var(--swagger-muted) !important;
  border-radius: 6px !important;
}

.swagger-ui .btn.btn-clear {
  background-color: #27272a !important;
  border: 1px solid #3f3f46 !important;
  color: #f4f4f5 !important;
  border-radius: 6px !important;
}

.swagger-ui .opblock-tag-section {
  margin-bottom: 24px;
}

.swagger-ui .opblock-tag {
  color: var(--swagger-text);
  border-bottom: 1px solid var(--swagger-border);
  font-size: 18px;
  font-weight: 600;
  padding: 12px 0;
}

.swagger-ui .opblock-tag small {
  color: var(--swagger-muted);
  font-size: 13px;
  font-weight: normal;
  margin-left: 10px;
}

.swagger-ui .opblock-tag svg {
  fill: var(--swagger-muted);
}

.swagger-ui .opblock {
  background-color: var(--swagger-card);
  border-radius: 8px;
  box-shadow: none;
  border: 1px solid var(--swagger-border);
  margin: 0 0 10px;
  overflow: hidden;
}

.swagger-ui .opblock .opblock-summary {
  padding: 10px 16px;
  border-color: transparent;
}

.swagger-ui .opblock .opblock-summary-method {
  border-radius: 6px;
  font-weight: 700;
  min-width: 68px;
  text-shadow: none;
  font-size: 12px;
  letter-spacing: 0.04em;
  padding: 5px 10px;
}

.swagger-ui .opblock .opblock-summary-path {
  color: var(--swagger-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 14px;
  font-weight: 600;
}

.swagger-ui .opblock .opblock-summary-path__deprecated {
  color: var(--swagger-muted);
  text-decoration: line-through;
}

.swagger-ui .opblock .opblock-summary-description {
  color: var(--swagger-muted);
  font-size: 13px;
}

.swagger-ui .opblock .opblock-body {
  background-color: var(--swagger-card);
  border-top: 1px solid var(--swagger-border);
}

.swagger-ui .opblock .opblock-section-header,
.swagger-ui .opblock-section-header {
  background: #18181b !important;
  background-color: #18181b !important;
  border-top: 1px solid var(--swagger-border) !important;
  border-bottom: 1px solid var(--swagger-border) !important;
  color: var(--swagger-text) !important;
  box-shadow: none !important;
}

.swagger-ui .opblock .opblock-section-header h4,
.swagger-ui .opblock-section-header h4 {
  color: var(--swagger-text) !important;
  font-weight: 600 !important;
}

.swagger-ui .opblock .opblock-section-header > label,
.swagger-ui .opblock-section-header > label {
  color: var(--swagger-muted) !important;
}

.swagger-ui .opblock .opblock-section-header .try-out__btn,
.swagger-ui .btn.try-out__btn {
  background-color: #27272a !important;
  border: 1px solid #3f3f46 !important;
  color: #f4f4f5 !important;
  border-radius: 6px !important;
  padding: 6px 14px !important;
  font-size: 12px !important;
  font-weight: 500 !important;
  box-shadow: none !important;
  transition: all 0.15s ease !important;
}

.swagger-ui .opblock .opblock-section-header .try-out__btn:hover,
.swagger-ui .btn.try-out__btn:hover {
  background-color: #3f3f46 !important;
  color: #ffffff !important;
}

.swagger-ui .opblock .opblock-section-header .try-out__btn.cancel,
.swagger-ui .btn.try-out__btn.cancel {
  background-color: rgba(239, 68, 68, 0.15) !important;
  border-color: rgba(239, 68, 68, 0.4) !important;
  color: #f87171 !important;
}

.swagger-ui .tabheader,
.swagger-ui .tab-item,
.swagger-ui .tabheader-title {
  color: var(--swagger-text) !important;
}

.swagger-ui .opblock .tab-header {
  border-bottom: 1px solid var(--swagger-border) !important;
}

.swagger-ui .responses-wrapper,
.swagger-ui .responses-inner {
  background-color: var(--swagger-card) !important;
}

.swagger-ui .parameters-container {
  background-color: var(--swagger-card) !important;
}

.swagger-ui .opblock.opblock-get {
  border-color: rgba(59, 130, 246, 0.35);
  background: rgba(59, 130, 246, 0.05);
}
.swagger-ui .opblock.opblock-get .opblock-summary-method {
  background: #2563eb;
  color: #ffffff;
}

.swagger-ui .opblock.opblock-post {
  border-color: rgba(16, 185, 129, 0.35);
  background: rgba(16, 185, 129, 0.05);
}
.swagger-ui .opblock.opblock-post .opblock-summary-method {
  background: #059669;
  color: #ffffff;
}

.swagger-ui .opblock.opblock-put {
  border-color: rgba(245, 158, 11, 0.35);
  background: rgba(245, 158, 11, 0.05);
}
.swagger-ui .opblock.opblock-put .opblock-summary-method {
  background: #d97706;
  color: #ffffff;
}

.swagger-ui .opblock.opblock-delete {
  border-color: rgba(239, 68, 68, 0.35);
  background: rgba(239, 68, 68, 0.05);
}
.swagger-ui .opblock.opblock-delete .opblock-summary-method {
  background: #dc2626;
  color: #ffffff;
}

.swagger-ui .opblock.opblock-patch {
  border-color: rgba(6, 182, 212, 0.35);
  background: rgba(6, 182, 212, 0.05);
}
.swagger-ui .opblock.opblock-patch .opblock-summary-method {
  background: #0891b2;
  color: #ffffff;
}

.swagger-ui table thead tr td,
.swagger-ui table thead tr th {
  color: var(--swagger-text);
  border-bottom: 1px solid var(--swagger-border);
}

.swagger-ui table tbody tr td {
  border-color: var(--swagger-border);
  color: var(--swagger-text);
}

.swagger-ui .parameters-col_name {
  color: var(--swagger-text);
}

.swagger-ui .parameter__name {
  color: var(--swagger-text);
  font-weight: 600;
}

.swagger-ui .parameter__type {
  color: var(--swagger-muted);
}

.swagger-ui .parameter__in {
  color: var(--swagger-muted);
  font-style: italic;
}

.swagger-ui .parameter__extension,
.swagger-ui .parameter__deprecated {
  color: #f87171;
}

.swagger-ui input[type=text],
.swagger-ui input[type=password],
.swagger-ui input[type=search],
.swagger-ui input[type=email],
.swagger-ui textarea,
.swagger-ui select {
  background-color: #18181b !important;
  border: 1px solid var(--swagger-border) !important;
  color: var(--swagger-text) !important;
  border-radius: 6px !important;
  padding: 8px 12px !important;
}

.swagger-ui input[type=text]:focus,
.swagger-ui textarea:focus,
.swagger-ui select:focus {
  border-color: var(--swagger-primary) !important;
  box-shadow: 0 0 0 1px var(--swagger-primary) !important;
  outline: none !important;
}

.swagger-ui .response-col_status {
  color: var(--swagger-text);
  font-weight: 600;
}

.swagger-ui .response-col_description {
  color: var(--swagger-muted);
}

.swagger-ui .responses-inner h4,
.swagger-ui .responses-inner h5 {
  color: var(--swagger-text);
}

.swagger-ui .highlight-code,
.swagger-ui .microlight,
.swagger-ui pre {
  background-color: #09090b !important;
  border-radius: 6px;
  border: 1px solid var(--swagger-border);
  color: #e4e4e7 !important;
}

.swagger-ui code {
  color: var(--swagger-primary);
}

.swagger-ui section.models {
  border: 1px solid var(--swagger-border);
  border-radius: 8px;
  background-color: var(--swagger-card);
  margin: 30px 0;
}

.swagger-ui section.models h4 {
  color: var(--swagger-text);
  border-bottom: 1px solid var(--swagger-border);
  padding: 12px 16px;
}

.swagger-ui section.models svg {
  fill: var(--swagger-muted);
}

.swagger-ui .model-box {
  background-color: transparent;
}

.swagger-ui .model {
  color: var(--swagger-text);
}

.swagger-ui .model-title {
  color: var(--swagger-text);
}

.swagger-ui .prop-type {
  color: var(--swagger-primary);
}

.swagger-ui .prop-format {
  color: var(--swagger-muted);
}

.swagger-ui .model-toggle:after {
  filter: invert(0.8);
}

.swagger-ui .dialog-ux .modal-ux {
  background-color: var(--swagger-card);
  border: 1px solid var(--swagger-border);
  border-radius: 10px;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.6);
}

.swagger-ui .dialog-ux .modal-ux-header {
  border-bottom: 1px solid var(--swagger-border);
  padding: 16px 20px;
}

.swagger-ui .dialog-ux .modal-ux-header h3 {
  color: var(--swagger-text);
  font-size: 18px;
  font-weight: 600;
}

.swagger-ui .dialog-ux .modal-ux-header .close-modal {
  fill: var(--swagger-muted);
}

.swagger-ui .dialog-ux .modal-ux-content {
  color: var(--swagger-text);
  padding: 20px;
}

.swagger-ui .dialog-ux .modal-ux-content h4 {
  color: var(--swagger-text);
}

.swagger-ui .dialog-ux .modal-ux-content p {
  color: var(--swagger-muted);
}

.swagger-ui .dialog-ux .backdrop-ux {
  background-color: rgba(0, 0, 0, 0.8);
  backdrop-filter: blur(2px);
}

.swagger-ui .scopes h2 {
  color: var(--swagger-text);
}
`)

	if opts.CustomCSS != "" {
		css.WriteString("\n/* User Custom CSS */\n")
		css.WriteString(opts.CustomCSS)
		css.WriteString("\n")
	}

	return []byte(css.String())
}
