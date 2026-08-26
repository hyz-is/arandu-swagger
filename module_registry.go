package swagger

import "github.com/arandu-io/hesape/jsonschema"

// Schema registers a reusable native Hesape JSON Schema component on this
// module instance.
func (m *Module) Schema(name string, schema jsonschema.Type) error {
	return m.registry.Schema(name, schema)
}

// Parameter registers a reusable OpenAPI parameter component on this module
// instance.
func (m *Module) Parameter(name string, parameter Parameter) error {
	return m.registry.Parameter(name, parameter)
}

// Response registers a reusable OpenAPI response component on this module
// instance.
func (m *Module) Response(name string, response Response) error {
	return m.registry.Response(name, response)
}

// RequestBody registers a reusable OpenAPI request-body component on this
// module instance.
func (m *Module) RequestBody(name string, body RequestBody) error {
	return m.registry.RequestBody(name, body)
}

// Header registers a reusable OpenAPI header component on this module instance.
func (m *Module) Header(name string, header Header) error {
	return m.registry.Header(name, header)
}

// ExampleComponent registers a reusable OpenAPI example component on this
// module instance.
func (m *Module) ExampleComponent(name string, example Example) error {
	return m.registry.ExampleComponent(name, example)
}

// SecurityScheme registers a reusable OpenAPI security scheme component on
// this module instance.
func (m *Module) SecurityScheme(name string, scheme SecurityScheme) error {
	return m.registry.SecurityScheme(name, scheme)
}
