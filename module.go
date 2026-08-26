// Package swagger generates and serves an OpenAPI document from the routes
// registered in an Arandu application.
package swagger

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/arandu-io/framework/foundation"
	fhttp "github.com/arandu-io/framework/http"

	"github.com/hyz-is/arandu-swagger/internal/ui"
)

const uiContentSecurityPolicy = "default-src 'none'; style-src 'self'; style-src-attr 'unsafe-inline'; script-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"

var (
	_ foundation.Module     = (*Module)(nil)
	_ foundation.Diagnostic = (*Module)(nil)
	_ Documenter            = (*Module)(nil)
)

// Module is the Arandu module that owns one isolated documentation registry.
// It captures the application's shared router when Routes is called and
// generates the document lazily, after every application module has had an
// opportunity to register its routes.
type Module struct {
	cfg      Config
	registry *Registry

	routerMu sync.RWMutex
	router   *fhttp.Router

	generationMu sync.Mutex
	cache        specificationCache

	diagnosticMu sync.RWMutex
	lastError    error
}

type specificationCache struct {
	revision         uint64
	routeFingerprint [sha256.Size]byte
	data             []byte
	err              error
	valid            bool
}

// New returns an independently configured Swagger module.
func New(cfg Config) (*Module, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Module{
		cfg:      cloneConfig(cfg.withDefaults()),
		registry: NewRegistry(),
	}, nil
}

// Name returns the stable module identifier.
func (m *Module) Name() string { return "swagger" }

// Route returns the operation builder associated with an application route.
func (m *Module) Route(route *fhttp.Route) *OperationBuilder {
	return m.registry.Route(route)
}

// Routes captures the shared application router and registers the enabled
// specification and self-hosted UI endpoints.
func (m *Module) Routes(router *fhttp.Router) {
	m.generationMu.Lock()
	m.routerMu.Lock()
	m.router = router
	m.routerMu.Unlock()
	m.cache = specificationCache{}
	m.generationMu.Unlock()

	if router == nil || !m.cfg.Enabled {
		return
	}

	specMiddleware := frameworkMiddleware(m.cfg.SpecMiddleware)
	uiMiddleware := frameworkMiddleware(m.cfg.UIMiddleware)
	if !m.cfg.DisableSpec {
		router.Get(m.cfg.SpecPath, m.serveSpecification, specMiddleware...).Name(documentationRouteName("spec", m.cfg.SpecPath))
	}
	if m.cfg.DisableUI {
		return
	}

	router.Get(m.cfg.UIPath, m.serveUI, uiMiddleware...).Name(documentationRouteName("ui", m.cfg.UIPath))
	redirectPath := m.cfg.UIPath + "/{$}"
	router.Get(redirectPath, m.redirectUI, uiMiddleware...).Name(documentationRouteName("ui.redirect", redirectPath))
	initializerPath := m.cfg.UIPath + "/swagger-initializer.js"
	router.Get(initializerPath, m.serveInitializer, uiMiddleware...).Name(documentationRouteName("ui.initializer", initializerPath))

	for _, assetPath := range ui.Paths() {
		fullPath := m.cfg.UIPath + "/" + assetPath
		router.Get(fullPath, m.serveAsset(assetPath), uiMiddleware...).Name(documentationRouteName("ui.asset", fullPath))
	}
}

// Generate builds the current typed OpenAPI document from the captured router.
func (m *Module) Generate() (*Document, error) {
	router, err := m.capturedRouter()
	if err != nil {
		m.setLastError(err)
		return nil, err
	}

	document, err := Generate(router.Routes(), m.registry, m.cfg)
	m.setLastError(err)
	return document, err
}

// GenerateJSON serializes the current OpenAPI document. When CacheSpec is
// enabled it shares the registry- and route-aware cache used by the HTTP
// endpoint.
func (m *Module) GenerateJSON() ([]byte, error) {
	return m.specificationBytes()
}

// Diagnose reports the most recent document generation failure.
func (m *Module) Diagnose(context.Context) []string {
	m.diagnosticMu.RLock()
	defer m.diagnosticMu.RUnlock()
	if m.lastError == nil {
		return nil
	}
	return []string{m.lastError.Error()}
}

func (m *Module) capturedRouter() (*fhttp.Router, error) {
	m.routerMu.RLock()
	router := m.router
	m.routerMu.RUnlock()
	if router == nil {
		return nil, errors.New("swagger: Routes must be called before generating the OpenAPI document")
	}
	return router, nil
}

func (m *Module) specificationBytes() ([]byte, error) {
	if !m.cfg.CacheSpec {
		router, err := m.capturedRouter()
		if err != nil {
			m.setLastError(err)
			return nil, err
		}
		data, generationErr := GenerateJSON(router.Routes(), m.registry, m.cfg)
		m.setLastError(generationErr)
		return bytes.Clone(data), generationErr
	}

	m.generationMu.Lock()
	defer m.generationMu.Unlock()

	router, err := m.capturedRouter()
	if err != nil {
		m.setLastError(err)
		return nil, err
	}

	routes := router.Routes()
	revision := m.registry.Revision()
	routeFingerprint := fingerprintRoutes(routes)
	if m.cache.valid && m.cache.revision == revision && m.cache.routeFingerprint == routeFingerprint {
		m.setLastError(m.cache.err)
		return bytes.Clone(m.cache.data), m.cache.err
	}

	// A builder may change while a document is being generated. Only cache a
	// result when its beginning and ending revisions agree; otherwise retry so
	// bytes are never stored under a revision they do not describe.
	var data []byte
	var generationErr error
	for attempts := 0; attempts < 3; attempts++ {
		revision = m.registry.Revision()
		routes = router.Routes()
		routeFingerprint = fingerprintRoutes(routes)
		data, generationErr = GenerateJSON(routes, m.registry, m.cfg)
		if revision == m.registry.Revision() && routeFingerprint == fingerprintRoutes(router.Routes()) {
			m.cache = specificationCache{
				revision:         revision,
				routeFingerprint: routeFingerprint,
				data:             bytes.Clone(data),
				err:              generationErr,
				valid:            true,
			}
			break
		}
	}

	m.setLastError(generationErr)
	return bytes.Clone(data), generationErr
}

func fingerprintRoutes(routes []*fhttp.Route) [sha256.Size]byte {
	digest := sha256.New()
	write := func(value string) {
		_, _ = fmt.Fprintf(digest, "%d:", len(value))
		_, _ = digest.Write([]byte(value))
		_, _ = digest.Write([]byte{0})
	}

	for _, route := range routes {
		if route == nil {
			write("<nil>")
			continue
		}
		write(fmt.Sprintf("%p", route))
		write(route.Method)
		write(route.URI())
		write(route.GetName())
		write(route.Module)
		write(route.GetDomain())
		write(fmt.Sprintf("%t", route.IsFallback()))
		write(strings.Join(route.Methods(), ","))
		if route.IsDeprecated() {
			since, sunset := route.GetDeprecation()
			write(since.String())
			write(sunset.String())
		}

		wheres := route.GetWheres()
		names := make([]string, 0, len(wheres))
		for name := range wheres {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			write(name)
			write(wheres[name])
		}
	}

	var fingerprint [sha256.Size]byte
	copy(fingerprint[:], digest.Sum(nil))
	return fingerprint
}

func (m *Module) setLastError(err error) {
	m.diagnosticMu.Lock()
	m.lastError = err
	m.diagnosticMu.Unlock()
}

func (m *Module) serveSpecification(w http.ResponseWriter, _ *http.Request) {
	setNoStoreHeaders(w, "application/json")

	data, err := m.specificationBytes()
	if err != nil {
		writeGenerationError(w)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (m *Module) serveUI(w http.ResponseWriter, _ *http.Request) {
	setNoStoreHeaders(w, "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", uiContentSecurityPolicy)
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(ui.Page(m.cfg.Title, m.cfg.UIPath))
}

func (m *Module) redirectUI(w http.ResponseWriter, request *http.Request) {
	setNoStoreHeaders(w, "text/html; charset=utf-8")
	http.Redirect(w, request, m.cfg.UIPath, http.StatusPermanentRedirect)
}

func (m *Module) serveInitializer(w http.ResponseWriter, _ *http.Request) {
	setNoStoreHeaders(w, "text/javascript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(ui.Initializer(m.cfg.SpecPath, m.cfg.PersistAuthorization, m.cfg.DisableTryItOut))
}

func (m *Module) serveAsset(assetPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, request *http.Request) {
		asset, ok := ui.Lookup(assetPath)
		if !ok {
			http.NotFound(w, request)
			return
		}

		w.Header().Set("Content-Type", asset.ContentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(asset.Bytes)
	}
}

func setNoStoreHeaders(w http.ResponseWriter, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
}

func writeGenerationError(w http.ResponseWriter) {
	setNoStoreHeaders(w, "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = fmt.Fprintln(w, `{"error":"failed to generate OpenAPI document"}`)
}

func frameworkMiddleware(middleware []Middleware) []fhttp.Middleware {
	converted := make([]fhttp.Middleware, len(middleware))
	for index, item := range middleware {
		converted[index] = fhttp.Middleware(item)
	}
	return converted
}

func documentationRouteName(kind, routePath string) string {
	var name strings.Builder
	name.WriteString("swagger.")
	name.WriteString(kind)
	separator := false
	for _, character := range routePath {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' {
			if separator {
				name.WriteByte('.')
				separator = false
			}
			name.WriteRune(character)
			continue
		}
		separator = name.Len() > 0
	}
	return strings.TrimSuffix(name.String(), ".")
}
