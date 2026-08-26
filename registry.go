package swagger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/jsonschema"
)

var componentNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// Documenter is the smallest contract an application module needs in order to
// document the routes it registers.
type Documenter interface {
	Route(route *fhttp.Route) *OperationBuilder
}

// Registry owns the route documentation and reusable OpenAPI components for
// one Swagger module instance.
//
// A Registry is safe for concurrent registration and generation. Builders and
// native schemas are snapshotted at their registration boundary; callers must
// still finish mutating a Hesape route before the application starts serving.
type Registry struct {
	mu         sync.RWMutex
	routes     map[*fhttp.Route]*operationDraft
	components Components
	issues     []error
	revision   uint64
}

// NewRegistry returns an empty, instance-owned documentation registry.
func NewRegistry() *Registry {
	return &Registry{
		routes: make(map[*fhttp.Route]*operationDraft),
		components: Components{
			Schemas:         make(map[string]Schema),
			Responses:       make(map[string]*Response),
			Parameters:      make(map[string]*Parameter),
			Examples:        make(map[string]*Example),
			RequestBodies:   make(map[string]*RequestBody),
			Headers:         make(map[string]*Header),
			SecuritySchemes: make(map[string]*SecurityScheme),
		},
	}
}

// Revision returns the registry revision used to invalidate a generated
// document cache. Every successful mutation and every recorded error advances
// it.
func (r *Registry) Revision() uint64 {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.revision
}

// Route returns the fluent documentation builder associated with route.
// Calling Route twice for the same pointer continues the same operation draft.
func (r *Registry) Route(route *fhttp.Route) *OperationBuilder {
	if r == nil {
		return &OperationBuilder{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if route == nil {
		r.recordLocked(errors.New("swagger: Route needs a non-nil Arandu route"))
		return &OperationBuilder{registry: r}
	}
	if _, exists := r.routes[route]; !exists {
		r.routes[route] = &operationDraft{
			operationIDs: make(map[string]string),
			responses:    make(Responses),
			extensions:   make(Extensions),
			explicit:     true,
		}
		r.revision++
	}
	return &OperationBuilder{registry: r, route: route}
}

// Schema registers a named reusable native Hesape JSON Schema snapshot.
// Repeating the same component is idempotent; changing its content is an error.
func (r *Registry) Schema(name string, native jsonschema.Type) error {
	schema, err := SchemaFrom(native)
	if err != nil {
		return r.componentError("schema", name, err)
	}
	return r.registerSchema(name, schema)
}

// Parameter registers a named reusable OpenAPI parameter.
func (r *Registry) Parameter(name string, parameter Parameter) error {
	copy := cloneParameter(&parameter)
	return registerComponent(r, "parameter", name, copy, func() map[string]*Parameter {
		return r.components.Parameters
	})
}

// Response registers a named reusable OpenAPI response.
func (r *Registry) Response(name string, response Response) error {
	copy := cloneResponse(&response)
	return registerComponent(r, "response", name, copy, func() map[string]*Response {
		return r.components.Responses
	})
}

// RequestBody registers a named reusable OpenAPI request body.
func (r *Registry) RequestBody(name string, body RequestBody) error {
	copy := cloneRequestBody(&body)
	return registerComponent(r, "request body", name, copy, func() map[string]*RequestBody {
		return r.components.RequestBodies
	})
}

// Header registers a named reusable OpenAPI response header.
func (r *Registry) Header(name string, header Header) error {
	copy := cloneHeader(&header)
	return registerComponent(r, "header", name, copy, func() map[string]*Header {
		return r.components.Headers
	})
}

// ExampleComponent registers a named reusable OpenAPI example.
func (r *Registry) ExampleComponent(name string, example Example) error {
	copy := cloneExample(&example)
	return registerComponent(r, "example", name, copy, func() map[string]*Example {
		return r.components.Examples
	})
}

// SecurityScheme registers a named reusable OpenAPI security scheme.
func (r *Registry) SecurityScheme(name string, scheme SecurityScheme) error {
	copy := cloneSecurityScheme(&scheme)
	return registerComponent(r, "security scheme", name, copy, func() map[string]*SecurityScheme {
		return r.components.SecuritySchemes
	})
}

func (r *Registry) registerSchema(name string, schema Schema) error {
	if r == nil {
		return errors.New("swagger: cannot register a schema on a nil Registry")
	}
	if err := validateComponentName("schema", name); err != nil {
		return r.componentError("schema", name, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.components.Schemas[name]; ok {
		if equalSchemaJSON(existing, schema) {
			return nil
		}
		err := fmt.Errorf("swagger: schema component %q is already registered with different content", name)
		r.recordLocked(err)
		return err
	}
	r.components.Schemas[name] = cloneSchema(schema)
	r.revision++
	return nil
}

func registerComponent[T any](r *Registry, kind, name string, value *T, collection func() map[string]*T) error {
	if r == nil {
		return fmt.Errorf("swagger: cannot register a %s on a nil Registry", kind)
	}
	if err := validateComponentName(kind, name); err != nil {
		return r.componentError(kind, name, err)
	}
	if value == nil {
		return r.componentError(kind, name, errors.New("component value is nil"))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	items := collection()
	if existing, ok := items[name]; ok {
		if equalJSON(existing, value) {
			return nil
		}
		err := fmt.Errorf("swagger: %s component %q is already registered with different content", kind, name)
		r.recordLocked(err)
		return err
	}
	items[name] = value
	r.revision++
	return nil
}

func (r *Registry) componentError(kind, name string, cause error) error {
	if r == nil {
		return fmt.Errorf("swagger: %s component %q: %w", kind, name, cause)
	}
	err := fmt.Errorf("swagger: %s component %q: %w", kind, name, cause)
	r.mu.Lock()
	r.recordLocked(err)
	r.mu.Unlock()
	return err
}

func validateComponentName(kind, name string) error {
	if !componentNamePattern.MatchString(name) {
		return fmt.Errorf("%s component name must match %s", kind, componentNamePattern.String())
	}
	return nil
}

func (r *Registry) recordLocked(err error) {
	if err == nil {
		return
	}
	r.issues = append(r.issues, err)
	r.revision++
}

// OperationBuilder documents one registered route. Its methods are safe to
// call concurrently, although normal applications use it only during boot.
type OperationBuilder struct {
	registry *Registry
	route    *fhttp.Route
}

// OperationID sets the operation identifier for a single-method route.
// Multi-method routes should use OperationIDFor or automatic method-qualified
// identifiers.
func (b *OperationBuilder) OperationID(id string) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		draft.operationID = strings.TrimSpace(id)
		if draft.operationID == "" {
			return errors.New("operationId must not be blank")
		}
		return nil
	})
}

// OperationIDFor sets the operation identifier for one concrete HTTP method of
// a multi-method route.
func (b *OperationBuilder) OperationIDFor(method, id string) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		method = strings.ToUpper(strings.TrimSpace(method))
		id = strings.TrimSpace(id)
		if method == "" || id == "" {
			return errors.New("OperationIDFor needs a non-blank method and operationId")
		}
		draft.operationIDs[method] = id
		return nil
	})
}

// Summary sets the operation's short summary.
func (b *OperationBuilder) Summary(summary string) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		draft.summary = summary
		return nil
	})
}

// Description sets the operation's long description.
func (b *OperationBuilder) Description(description string) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		draft.description = description
		return nil
	})
}

// Tags replaces the operation's explicit tags.
func (b *OperationBuilder) Tags(tags ...string) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		seen := make(map[string]bool, len(tags))
		draft.tags = draft.tags[:0]
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				return errors.New("operation tag must not be blank")
			}
			if !seen[tag] {
				draft.tags = append(draft.tags, tag)
				seen[tag] = true
			}
		}
		return nil
	})
}

// ExternalDocumentation attaches further documentation to the operation.
func (b *OperationBuilder) ExternalDocumentation(docs ExternalDocumentation) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		copy := cloneExternalDocs(&docs)
		draft.externalDocs = copy
		return nil
	})
}

// PathParameter documents a parameter already present in the route path.
func (b *OperationBuilder) PathParameter(name string, options ...ParameterOption) *OperationBuilder {
	return b.parameter(name, ParameterInPath, nil, options...)
}

// QueryParameter documents a query-string parameter with a native schema.
func (b *OperationBuilder) QueryParameter(name string, schema jsonschema.Type, options ...ParameterOption) *OperationBuilder {
	return b.parameter(name, ParameterInQuery, schema, options...)
}

// HeaderParameter documents a request-header parameter with a native schema.
func (b *OperationBuilder) HeaderParameter(name string, schema jsonschema.Type, options ...ParameterOption) *OperationBuilder {
	return b.parameter(name, ParameterInHeader, schema, options...)
}

// CookieParameter documents a cookie parameter with a native schema.
func (b *OperationBuilder) CookieParameter(name string, schema jsonschema.Type, options ...ParameterOption) *OperationBuilder {
	return b.parameter(name, ParameterInCookie, schema, options...)
}

// ParameterRef adds a reference to a reusable parameter component.
func (b *OperationBuilder) ParameterRef(name string) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		if err := validateComponentName("parameter", name); err != nil {
			return err
		}
		draft.parameters = append(draft.parameters, &Parameter{Ref: "#/components/parameters/" + escapeJSONPointerToken(name)})
		return nil
	})
}

func (b *OperationBuilder) parameter(name string, location ParameterLocation, native jsonschema.Type, options ...ParameterOption) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("%s parameter name must not be blank", location)
		}
		parameter := &Parameter{Name: name, In: location, Required: location == ParameterInPath}
		if native != nil {
			schema, err := SchemaFrom(native)
			if err != nil {
				return fmt.Errorf("%s parameter %q schema: %w", location, name, err)
			}
			parameter.Schema = &schema
		} else if location != ParameterInPath {
			return fmt.Errorf("%s parameter %q needs a schema", location, name)
		}
		settings := parameterSettings{parameter: parameter}
		for _, option := range options {
			if option == nil {
				return fmt.Errorf("%s parameter %q has a nil option", location, name)
			}
			if err := option(&settings); err != nil {
				return fmt.Errorf("%s parameter %q: %w", location, name, err)
			}
		}
		draft.parameters = append(draft.parameters, cloneParameter(parameter))
		return nil
	})
}

// RequestBody sets the operation request body from one or more media types.
func (b *OperationBuilder) RequestBody(media ...*Media) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		content, required, err := buildContent(media)
		if err != nil {
			return fmt.Errorf("request body: %w", err)
		}
		draft.requestBody = &RequestBody{Content: content, Required: required}
		return nil
	})
}

// RequestBodyRef sets the operation request body to a reusable request-body
// component.
func (b *OperationBuilder) RequestBodyRef(component string) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		if err := validateComponentName("request body", component); err != nil {
			return err
		}
		draft.requestBody = &RequestBody{Ref: "#/components/requestBodies/" + escapeJSONPointerToken(component)}
		return nil
	})
}

// RequestBodyDescription sets the request body description, creating an empty
// body draft when necessary so generation can report missing content clearly.
func (b *OperationBuilder) RequestBodyDescription(description string) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		if draft.requestBody == nil {
			draft.requestBody = &RequestBody{}
		}
		draft.requestBody.Description = description
		return nil
	})
}

// Response declares a concrete HTTP status response.
func (b *OperationBuilder) Response(status int, description string, media ...*Media) *OperationBuilder {
	return b.response(strconv.Itoa(status), description, media...)
}

// DefaultResponse declares the operation's default response.
func (b *OperationBuilder) DefaultResponse(description string, media ...*Media) *OperationBuilder {
	return b.response("default", description, media...)
}

func (b *OperationBuilder) response(status, description string, media ...*Media) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		if status != "default" {
			code, err := strconv.Atoi(status)
			if err != nil || code < 100 || code > 599 {
				return fmt.Errorf("response status %q must be default or an HTTP status from 100 through 599", status)
			}
		}
		content, _, err := buildContent(media)
		if err != nil {
			return fmt.Errorf("response %s: %w", status, err)
		}
		response := &Response{Description: description, Content: content}
		if existing := draft.responses[status]; existing != nil && len(existing.Headers) > 0 {
			response.Headers = cloneHeaders(existing.Headers)
		}
		draft.responses[status] = response
		return nil
	})
}

// ResponseRef declares a response that references a reusable response
// component.
func (b *OperationBuilder) ResponseRef(status int, component string) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		if status < 100 || status > 599 {
			return fmt.Errorf("response status %d must be from 100 through 599", status)
		}
		if err := validateComponentName("response", component); err != nil {
			return err
		}
		draft.responses[strconv.Itoa(status)] = &Response{Ref: "#/components/responses/" + escapeJSONPointerToken(component)}
		return nil
	})
}

// ResponseHeader adds a header to a previously declared response. It may be
// called first; generation then reports a missing response description.
func (b *OperationBuilder) ResponseHeader(status int, name string, header Header) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		if status < 100 || status > 599 {
			return fmt.Errorf("response status %d must be from 100 through 599", status)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return errors.New("response header name must not be blank")
		}
		key := strconv.Itoa(status)
		response := draft.responses[key]
		if response == nil {
			response = &Response{}
			draft.responses[key] = response
		}
		if response.Headers == nil {
			response.Headers = make(map[string]*Header)
		}
		if header.Ref == "" && header.Schema == nil && len(header.Content) == 0 {
			schema, err := SchemaFrom(jsonschema.String())
			if err != nil {
				return fmt.Errorf("response header %q: %w", name, err)
			}
			header.Schema = &schema
		}
		response.Headers[name] = cloneHeader(&header)
		return nil
	})
}

// Security adds one alternative security requirement. Calling it more than
// once creates OpenAPI OR alternatives; pass multiple schemes to SecurityAll
// for an AND requirement.
func (b *OperationBuilder) Security(name string, scopes ...string) *OperationBuilder {
	return b.SecurityAll(SecurityRequirementFor(name, scopes...))
}

// SecurityAll adds one security requirement containing every supplied scheme.
func (b *OperationBuilder) SecurityAll(requirement SecurityRequirement) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		if len(requirement) == 0 {
			return errors.New("security requirement must name at least one scheme")
		}
		copy := make(SecurityRequirement, len(requirement))
		for name, scopes := range requirement {
			if strings.TrimSpace(name) == "" {
				return errors.New("security scheme name must not be blank")
			}
			copy[name] = append(make([]string, 0, len(scopes)), scopes...)
		}
		draft.security = append(draft.security, copy)
		return nil
	})
}

// Deprecated marks the operation deprecated even when the route itself has no
// deprecation dates.
func (b *OperationBuilder) Deprecated() *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		draft.deprecated = true
		return nil
	})
}

// Hidden excludes the operation from every generated document.
func (b *OperationBuilder) Hidden() *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		draft.hidden = true
		return nil
	})
}

// Ignore is an alias for Hidden.
func (b *OperationBuilder) Ignore() *OperationBuilder { return b.Hidden() }

// Methods explicitly maps an ANY route, or a subset of a multi-method route,
// to concrete OpenAPI methods.
func (b *OperationBuilder) Methods(methods ...string) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		seen := make(map[string]bool, len(methods))
		draft.methods = draft.methods[:0]
		for _, method := range methods {
			method = strings.ToUpper(strings.TrimSpace(method))
			if method == "" {
				return errors.New("explicit operation method must not be blank")
			}
			if !seen[method] {
				draft.methods = append(draft.methods, method)
				seen[method] = true
			}
		}
		if len(draft.methods) == 0 {
			return errors.New("Methods needs at least one concrete HTTP method")
		}
		return nil
	})
}

// Extension sets an x-* operation extension after snapshotting its JSON value.
func (b *OperationBuilder) Extension(name string, value any) *OperationBuilder {
	return b.update(func(draft *operationDraft) error {
		if !strings.HasPrefix(name, "x-") || len(name) == 2 {
			return fmt.Errorf("extension key %q must start with x- and include a name", name)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("extension %q: %w", name, err)
		}
		draft.extensions[name] = bytes.Clone(encoded)
		return nil
	})
}

func (b *OperationBuilder) update(change func(*operationDraft) error) *OperationBuilder {
	if b == nil || b.registry == nil {
		return b
	}
	r := b.registry
	r.mu.Lock()
	defer r.mu.Unlock()
	draft := r.routes[b.route]
	if draft == nil {
		r.recordLocked(errors.New("swagger: operation builder is not attached to a route"))
		return b
	}
	if err := change(draft); err != nil {
		r.recordLocked(fmt.Errorf("swagger: operation %s: %w", routeLabel(b.route), err))
		return b
	}
	r.revision++
	return b
}

func routeLabel(route *fhttp.Route) string {
	if route == nil {
		return "<nil route>"
	}
	if name := route.GetName(); name != "" {
		return name
	}
	return route.Method + " " + route.URI()
}

// ParameterOption changes an explicitly documented operation parameter.
type ParameterOption func(*parameterSettings) error

type parameterSettings struct {
	parameter *Parameter
}

// Description returns an option that describes a parameter.
func Description(description string) ParameterOption {
	return func(settings *parameterSettings) error {
		settings.parameter.Description = description
		return nil
	}
}

// ExampleValue returns an option that snapshots one parameter example.
func ExampleValue(value any) ParameterOption {
	return func(settings *parameterSettings) error {
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("marshal example: %w", err)
		}
		settings.parameter.Example = bytes.Clone(encoded)
		return nil
	}
}

// ParameterSchema returns an option that uses a native Hesape schema.
func ParameterSchema(native jsonschema.Type) ParameterOption {
	return func(settings *parameterSettings) error {
		schema, err := SchemaFrom(native)
		if err != nil {
			return err
		}
		settings.parameter.Schema = &schema
		return nil
	}
}

// ParameterSchemaRef returns an option that references a reusable schema.
func ParameterSchemaRef(name string) ParameterOption {
	return func(settings *parameterSettings) error {
		schema := SchemaRef(name)
		settings.parameter.Schema = &schema
		return nil
	}
}

// ParameterRequired returns an option that marks a non-path parameter as
// required. Path parameters are always required regardless of this option.
func ParameterRequired() ParameterOption {
	return func(settings *parameterSettings) error {
		settings.parameter.Required = true
		return nil
	}
}

// ParameterDeprecated returns an option that marks a parameter deprecated.
func ParameterDeprecated() ParameterOption {
	return func(settings *parameterSettings) error {
		settings.parameter.Deprecated = true
		return nil
	}
}

// Media is one request or response representation supplied to a builder.
// Its schema and examples are immutable snapshots.
type Media struct {
	contentType string
	schema      Schema
	required    bool
	example     json.RawMessage
	examples    map[string]*Example
	err         error
}

// JSON returns an application/json representation using a native Hesape
// schema. Any snapshot failure is reported by generation with route context.
func JSON(native jsonschema.Type) *Media {
	schema, err := SchemaFrom(native)
	return &Media{contentType: "application/json", schema: schema, err: err}
}

// JSONRef returns an application/json representation referencing a named
// schema component.
func JSONRef(name string) *Media {
	return &Media{contentType: "application/json", schema: SchemaRef(name)}
}

// MediaOf returns a representation with an explicit content type and native
// Hesape schema.
func MediaOf(contentType string, native jsonschema.Type) *Media {
	schema, err := SchemaFrom(native)
	return &Media{contentType: strings.TrimSpace(contentType), schema: schema, err: err}
}

// MediaRef returns a representation with an explicit content type and schema
// component reference.
func MediaRef(contentType, component string) *Media {
	return &Media{contentType: strings.TrimSpace(contentType), schema: SchemaRef(component)}
}

// Required marks this representation's request body required. If any
// representation is required, the containing request body is required.
func (m *Media) Required() *Media {
	if m != nil {
		m.required = true
	}
	return m
}

// Example snapshots one example value for this representation.
func (m *Media) Example(value any) *Media {
	if m == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		m.err = errors.Join(m.err, fmt.Errorf("marshal media example: %w", err))
		return m
	}
	m.example = bytes.Clone(encoded)
	return m
}

// NamedExample snapshots a named example for this representation.
func (m *Media) NamedExample(name, summary, description string, value any) *Media {
	if m == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		m.err = errors.Join(m.err, errors.New("media example name must not be blank"))
		return m
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		m.err = errors.Join(m.err, fmt.Errorf("marshal media example %q: %w", name, err))
		return m
	}
	if m.examples == nil {
		m.examples = make(map[string]*Example)
	}
	m.examples[name] = &Example{Summary: summary, Description: description, Value: bytes.Clone(encoded)}
	return m
}

func buildContent(media []*Media) (Content, bool, error) {
	if len(media) == 0 {
		return nil, false, nil
	}
	content := make(Content, len(media))
	required := false
	for index, item := range media {
		if item == nil {
			return nil, false, fmt.Errorf("media[%d] is nil", index)
		}
		if item.err != nil {
			return nil, false, fmt.Errorf("media[%d] schema or example: %w", index, item.err)
		}
		if _, err := normalizeMediaType(item.contentType); err != nil {
			return nil, false, fmt.Errorf("media[%d] content type %q is invalid", index, item.contentType)
		}
		if item.schema.IsZero() {
			return nil, false, fmt.Errorf("media[%d] needs a schema", index)
		}
		if _, exists := content[item.contentType]; exists {
			return nil, false, fmt.Errorf("content type %q is declared more than once", item.contentType)
		}
		schema := cloneSchema(item.schema)
		content[item.contentType] = &MediaType{
			Schema:   &schema,
			Example:  bytes.Clone(item.example),
			Examples: cloneExamples(item.examples),
		}
		required = required || item.required
	}
	return content, required, nil
}

type operationDraft struct {
	operationID  string
	operationIDs map[string]string
	summary      string
	description  string
	tags         []string
	externalDocs *ExternalDocumentation
	parameters   []*Parameter
	requestBody  *RequestBody
	responses    Responses
	security     []SecurityRequirement
	deprecated   bool
	hidden       bool
	methods      []string
	extensions   Extensions
	explicit     bool
}

type registrySnapshot struct {
	routes     map[*fhttp.Route]operationDraft
	components Components
	issues     []error
	revision   uint64
}

func (r *Registry) snapshot() (registrySnapshot, error) {
	if r == nil {
		return registrySnapshot{}, errors.New("swagger: Generate needs a non-nil Registry")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshot := registrySnapshot{
		routes:     make(map[*fhttp.Route]operationDraft, len(r.routes)),
		components: cloneComponents(r.components),
		issues:     append([]error(nil), r.issues...),
		revision:   r.revision,
	}
	for route, draft := range r.routes {
		snapshot.routes[route] = cloneOperationDraft(draft)
	}
	return snapshot, nil
}

func cloneOperationDraft(source *operationDraft) operationDraft {
	if source == nil {
		return operationDraft{}
	}
	copy := operationDraft{
		operationID:  source.operationID,
		operationIDs: cloneStringMap(source.operationIDs),
		summary:      source.summary,
		description:  source.description,
		tags:         append([]string(nil), source.tags...),
		externalDocs: cloneExternalDocs(source.externalDocs),
		parameters:   cloneParameters(source.parameters),
		requestBody:  cloneRequestBody(source.requestBody),
		responses:    cloneResponses(source.responses),
		security:     cloneSecurityRequirements(source.security),
		deprecated:   source.deprecated,
		hidden:       source.hidden,
		methods:      append([]string(nil), source.methods...),
		extensions:   cloneExtensions(source.extensions),
		explicit:     source.explicit,
	}
	return copy
}

func equalJSON(left, right any) bool {
	a, errA := canonicalJSON(left)
	b, errB := canonicalJSON(right)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}

func equalSchemaJSON(left, right Schema) bool {
	a, errA := canonicalSchemaJSON(left)
	b, errB := canonicalSchemaJSON(right)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}

func canonicalSchemaJSON(schema Schema) ([]byte, error) {
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	normalizeSchemaSets(decoded)
	return json.Marshal(decoded)
}

func normalizeSchemaSets(value any) {
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	for _, keyword := range []string{"required", "enum", "type"} {
		if values, ok := object[keyword].([]any); ok {
			sort.SliceStable(values, func(i, j int) bool {
				left, _ := json.Marshal(values[i])
				right, _ := json.Marshal(values[j])
				return bytes.Compare(left, right) < 0
			})
		}
	}
	for _, keyword := range singleSubschemaKeywords {
		normalizeSchemaSets(object[keyword])
	}
	for _, keyword := range arraySubschemaKeywords {
		if schemas, ok := object[keyword].([]any); ok {
			for _, schema := range schemas {
				normalizeSchemaSets(schema)
			}
		}
	}
	for _, keyword := range mapSubschemaKeywords {
		if schemas, ok := object[keyword].(map[string]any); ok {
			for _, schema := range schemas {
				normalizeSchemaSets(schema)
			}
		}
	}
	if dependencies, ok := object["dependencies"].(map[string]any); ok {
		for _, dependency := range dependencies {
			normalizeSchemaSets(dependency)
		}
	}
}

func cloneSchema(source Schema) Schema {
	return Schema{json: bytes.Clone(source.json), reference: source.reference}
}

func cloneComponents(source Components) Components {
	copy := Components{
		Schemas:         make(map[string]Schema, len(source.Schemas)),
		Responses:       make(map[string]*Response, len(source.Responses)),
		Parameters:      make(map[string]*Parameter, len(source.Parameters)),
		Examples:        make(map[string]*Example, len(source.Examples)),
		RequestBodies:   make(map[string]*RequestBody, len(source.RequestBodies)),
		Headers:         make(map[string]*Header, len(source.Headers)),
		SecuritySchemes: make(map[string]*SecurityScheme, len(source.SecuritySchemes)),
		Extensions:      cloneExtensions(source.Extensions),
	}
	for name, schema := range source.Schemas {
		copy.Schemas[name] = cloneSchema(schema)
	}
	for name, response := range source.Responses {
		copy.Responses[name] = cloneResponse(response)
	}
	for name, parameter := range source.Parameters {
		copy.Parameters[name] = cloneParameter(parameter)
	}
	for name, example := range source.Examples {
		copy.Examples[name] = cloneExample(example)
	}
	for name, body := range source.RequestBodies {
		copy.RequestBodies[name] = cloneRequestBody(body)
	}
	for name, header := range source.Headers {
		copy.Headers[name] = cloneHeader(header)
	}
	for name, scheme := range source.SecuritySchemes {
		copy.SecuritySchemes[name] = cloneSecurityScheme(scheme)
	}
	return copy
}

func cloneParameter(source *Parameter) *Parameter {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Schema = cloneSchemaPointer(source.Schema)
	copy.Explode = cloneBoolPointer(source.Explode)
	copy.Example = bytes.Clone(source.Example)
	copy.Examples = cloneExamples(source.Examples)
	copy.Content = cloneContent(source.Content)
	copy.Extensions = cloneExtensions(source.Extensions)
	return &copy
}

func cloneParameters(source []*Parameter) []*Parameter {
	copy := make([]*Parameter, len(source))
	for index, parameter := range source {
		copy[index] = cloneParameter(parameter)
	}
	return copy
}

func cloneHeader(source *Header) *Header {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Schema = cloneSchemaPointer(source.Schema)
	copy.Explode = cloneBoolPointer(source.Explode)
	copy.Example = bytes.Clone(source.Example)
	copy.Examples = cloneExamples(source.Examples)
	copy.Content = cloneContent(source.Content)
	copy.Extensions = cloneExtensions(source.Extensions)
	return &copy
}

func cloneHeaders(source map[string]*Header) map[string]*Header {
	if source == nil {
		return nil
	}
	copy := make(map[string]*Header, len(source))
	for name, header := range source {
		copy[name] = cloneHeader(header)
	}
	return copy
}

func cloneRequestBody(source *RequestBody) *RequestBody {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Content = cloneContent(source.Content)
	copy.Extensions = cloneExtensions(source.Extensions)
	return &copy
}

func cloneResponse(source *Response) *Response {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Headers = cloneHeaders(source.Headers)
	copy.Content = cloneContent(source.Content)
	copy.Extensions = cloneExtensions(source.Extensions)
	return &copy
}

func cloneResponses(source Responses) Responses {
	if source == nil {
		return nil
	}
	copy := make(Responses, len(source))
	for status, response := range source {
		copy[status] = cloneResponse(response)
	}
	return copy
}

func cloneContent(source Content) Content {
	if source == nil {
		return nil
	}
	copy := make(Content, len(source))
	for contentType, media := range source {
		if media == nil {
			copy[contentType] = nil
			continue
		}
		cloned := *media
		cloned.Schema = cloneSchemaPointer(media.Schema)
		cloned.Example = bytes.Clone(media.Example)
		cloned.Examples = cloneExamples(media.Examples)
		cloned.Encoding = cloneEncodings(media.Encoding)
		cloned.Extensions = cloneExtensions(media.Extensions)
		copy[contentType] = &cloned
	}
	return copy
}

func cloneEncodings(source map[string]*Encoding) map[string]*Encoding {
	if source == nil {
		return nil
	}
	copy := make(map[string]*Encoding, len(source))
	for name, encoding := range source {
		if encoding == nil {
			copy[name] = nil
			continue
		}
		cloned := *encoding
		cloned.Explode = cloneBoolPointer(encoding.Explode)
		cloned.Headers = cloneHeaders(encoding.Headers)
		cloned.Extensions = cloneExtensions(encoding.Extensions)
		copy[name] = &cloned
	}
	return copy
}

func cloneExample(source *Example) *Example {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Value = bytes.Clone(source.Value)
	copy.Extensions = cloneExtensions(source.Extensions)
	return &copy
}

func cloneExamples(source map[string]*Example) map[string]*Example {
	if source == nil {
		return nil
	}
	copy := make(map[string]*Example, len(source))
	for name, example := range source {
		copy[name] = cloneExample(example)
	}
	return copy
}

func cloneSecurityScheme(source *SecurityScheme) *SecurityScheme {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Flows = cloneOAuthFlows(source.Flows)
	copy.Extensions = cloneExtensions(source.Extensions)
	return &copy
}

func cloneOAuthFlows(source *OAuthFlows) *OAuthFlows {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Implicit = cloneOAuthFlow(source.Implicit)
	copy.Password = cloneOAuthFlow(source.Password)
	copy.ClientCredentials = cloneOAuthFlow(source.ClientCredentials)
	copy.AuthorizationCode = cloneOAuthFlow(source.AuthorizationCode)
	copy.Extensions = cloneExtensions(source.Extensions)
	return &copy
}

func cloneOAuthFlow(source *OAuthFlow) *OAuthFlow {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Scopes = cloneStringMap(source.Scopes)
	copy.Extensions = cloneExtensions(source.Extensions)
	return &copy
}

func cloneExternalDocs(source *ExternalDocumentation) *ExternalDocumentation {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Extensions = cloneExtensions(source.Extensions)
	return &copy
}

func cloneSchemaPointer(source *Schema) *Schema {
	if source == nil {
		return nil
	}
	copy := cloneSchema(*source)
	return &copy
}

func cloneBoolPointer(source *bool) *bool {
	if source == nil {
		return nil
	}
	copy := *source
	return &copy
}

func cloneExtensions(source Extensions) Extensions {
	if source == nil {
		return nil
	}
	copy := make(Extensions, len(source))
	for name, value := range source {
		copy[name] = bytes.Clone(value)
	}
	return copy
}

func cloneSecurityRequirements(source []SecurityRequirement) []SecurityRequirement {
	if source == nil {
		return nil
	}
	copy := make([]SecurityRequirement, len(source))
	for index, requirement := range source {
		copy[index] = make(SecurityRequirement, len(requirement))
		for name, scopes := range requirement {
			copy[index][name] = append(make([]string, 0, len(scopes)), scopes...)
		}
	}
	return copy
}

func cloneStringMap[T ~string](source map[string]T) map[string]T {
	if source == nil {
		return nil
	}
	copy := make(map[string]T, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
