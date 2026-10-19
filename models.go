package swagger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Model names a Go type as a schema component: the name the document gives
// it, and the type whose values a response or a request carries.
//
// A component derived from the type an application answers with is the
// contract and the answer at once. There is no second description of the
// payload to keep in step with the struct, which is the drift a hand-written
// schema invites: a field renamed in Go and not in the document is a client
// generated against a property the server stopped sending.
type Model struct {
	Name string
	Type reflect.Type
}

// ModelOf names T as the schema component name.
//
//	swagger.ModelOf[ProductResource]("Product")
func ModelOf[T any](name string) Model {
	return Model{Name: name, Type: reflect.TypeFor[T]()}
}

// SchemasFromModels derives the schema component of each model from its Go
// type, the way encoding/json writes a value of it:
//
//   - a struct is an object whose properties are its exported fields, named
//     and skipped by their json tags, in declaration order, with the fields of
//     an embedded struct promoted as encoding/json promotes them. A field
//     whose json tag carries neither omitempty nor omitzero is always written,
//     so it is required;
//   - string is string, bool is boolean, every integer kind is integer, float32
//     and float64 are number, and time.Time is a string of format date-time;
//   - a slice or an array is an array of its element; a map with string keys
//     is an object whose additionalProperties is its value; json.RawMessage and
//     an interface accept any value;
//   - a pointer is its element or null: ["string","null"] for a scalar, and
//     anyOf the element and null for an object, an array or a reference;
//   - a struct type that another model names is a $ref to that component
//     wherever it appears below the root, and an unnamed struct is inlined.
//
// Three struct tags describe a field beyond its type: doc is its description,
// format its format (date-time, uri, email, ...), and enum the values it takes,
// separated by commas -- integers for an integer field.
//
// A type it cannot say -- a channel, a function, a complex number, a map with
// keys that are not strings, an inline struct that contains itself -- and a
// json tag option it does not follow, such as string, are errors naming the
// model and the field: a schema that guessed would describe something the
// encoder does not write.
//
// A nil slice or map is written by encoding/json as null, and the schema says
// array or object: an application whose answers can carry one fills it, or
// declares the field as a pointer.
//
// Every schema it returns passed the same validation SchemaFromJSON applies.
func SchemasFromModels(models ...Model) (map[string]Schema, error) {
	names := make(map[reflect.Type]string, len(models))
	for _, model := range models {
		if model.Type == nil {
			return nil, fmt.Errorf("swagger: model %q has no Go type", model.Name)
		}
		if err := validateComponentName("schema", model.Name); err != nil {
			return nil, fmt.Errorf("swagger: model %q: %w", model.Name, err)
		}
		if other, taken := names[model.Type]; taken {
			return nil, fmt.Errorf("swagger: models %q and %q name the same Go type %s", other, model.Name, model.Type)
		}
		names[model.Type] = model.Name
	}
	out := make(map[string]Schema, len(models))
	for _, model := range models {
		if _, taken := out[model.Name]; taken {
			return nil, fmt.Errorf("swagger: two models are named %q", model.Name)
		}
		d := deriver{names: names, inline: map[reflect.Type]bool{}}
		node, err := d.root(model.Type)
		if err != nil {
			return nil, fmt.Errorf("swagger: model %q: %w", model.Name, err)
		}
		encoded, err := node.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("swagger: model %q: %w", model.Name, err)
		}
		schema, err := SchemaFromJSON(encoded)
		if err != nil {
			return nil, fmt.Errorf("swagger: model %q: %w", model.Name, err)
		}
		out[model.Name] = schema
	}
	return out, nil
}

// Models registers the schema component SchemasFromModels derives from each
// model's Go type. Nothing is registered when any of them cannot be derived.
func (r *Registry) Models(models ...Model) error {
	schemas, err := SchemasFromModels(models...)
	if err != nil {
		if r != nil {
			r.mu.Lock()
			r.recordLocked(err)
			r.mu.Unlock()
		}
		return err
	}
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := r.registerSchema(name, schemas[name]); err != nil {
			return err
		}
	}
	return nil
}

// Models registers the schema components derived from the models' Go types
// on this module instance.
func (m *Module) Models(models ...Model) error {
	return m.registry.Models(models...)
}

var (
	timeType       = reflect.TypeFor[time.Time]()
	rawMessageType = reflect.TypeFor[json.RawMessage]()
)

// deriver walks one model's type. inline holds the unnamed struct types being
// written, so one that contains itself is refused instead of recursing.
type deriver struct {
	names  map[reflect.Type]string
	inline map[reflect.Type]bool
}

// root derives the model's own type, which is written out even when it is
// the type a reference elsewhere would name.
func (d deriver) root(t reflect.Type) (*schemaNode, error) {
	if t.Kind() == reflect.Struct && t != timeType {
		return d.object(t)
	}
	return d.of(t, fieldTags{})
}

// of derives the schema of a value of t.
func (d deriver) of(t reflect.Type, tags fieldTags) (*schemaNode, error) {
	if t.Kind() == reflect.Pointer {
		inner, err := d.of(t.Elem(), tags)
		if err != nil {
			return nil, err
		}
		return inner.nullable(), nil
	}
	if name, named := d.names[t]; named {
		return &schemaNode{ref: schemaComponentPrefix + escapeJSONPointerToken(name), description: tags.doc}, nil
	}
	node := &schemaNode{format: tags.format, description: tags.doc}
	switch {
	case t == timeType:
		node.kind = "string"
		if node.format == "" {
			node.format = "date-time"
		}
	case t == rawMessageType || t.Kind() == reflect.Interface:
		node.any = true
	default:
		switch t.Kind() {
		case reflect.String:
			node.kind = "string"
		case reflect.Bool:
			node.kind = "boolean"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			node.kind = "integer"
		case reflect.Float32, reflect.Float64:
			node.kind = "number"
		case reflect.Slice, reflect.Array:
			if t.Elem().Kind() == reflect.Uint8 && t.Kind() == reflect.Slice {
				return nil, fmt.Errorf("%s is written by encoding/json as base64 text, which this derivation does not describe", t)
			}
			items, err := d.of(t.Elem(), fieldTags{})
			if err != nil {
				return nil, err
			}
			node.kind, node.items = "array", items
		case reflect.Map:
			if t.Key().Kind() != reflect.String {
				return nil, fmt.Errorf("%s has keys that are not strings, which a JSON object cannot hold", t)
			}
			values, err := d.of(t.Elem(), fieldTags{})
			if err != nil {
				return nil, err
			}
			node.kind, node.additional = "object", values
		case reflect.Struct:
			if d.inline[t] {
				return nil, fmt.Errorf("the unnamed struct %s contains itself; name it as a model to refer to it", t)
			}
			object, err := d.object(t)
			if err != nil {
				return nil, err
			}
			object.description = tags.doc
			return object, nil
		default:
			return nil, fmt.Errorf("%s has no JSON Schema", t)
		}
	}
	if len(tags.enum) > 0 {
		values, err := enumValues(node.kind, tags.enum)
		if err != nil {
			return nil, err
		}
		node.enum = values
	}
	return node, nil
}

// object derives a struct as encoding/json writes it.
func (d deriver) object(t reflect.Type) (*schemaNode, error) {
	d.inline[t] = true
	defer delete(d.inline, t)
	node := &schemaNode{kind: "object", object: true}
	seen := map[string]bool{}
	if err := d.fields(t, node, seen); err != nil {
		return nil, err
	}
	return node, nil
}

// fields adds the properties a struct's fields write, promoting the fields of
// an embedded struct that has no json name of its own.
func (d deriver) fields(t reflect.Type, node *schemaNode, seen map[string]bool) error {
	for i := range t.NumField() {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, options, _ := strings.Cut(tag, ",")
		embedded := field.Anonymous && name == ""
		if embedded {
			inner := field.Type
			if inner.Kind() == reflect.Pointer {
				inner = inner.Elem()
			}
			if inner.Kind() == reflect.Struct && inner != timeType {
				if err := d.fields(inner, node, seen); err != nil {
					return err
				}
				continue
			}
		}
		if !field.IsExported() {
			continue
		}
		if name == "" {
			name = field.Name
		}
		optional := false
		for _, option := range strings.Split(options, ",") {
			switch option {
			case "":
			case "omitempty", "omitzero":
				optional = true
			default:
				return fmt.Errorf("field %s.%s: the json option %q is not described", t, field.Name, option)
			}
		}
		if seen[name] {
			return fmt.Errorf("field %s.%s: the property %q is written twice", t, field.Name, name)
		}
		seen[name] = true
		tags := fieldTags{doc: field.Tag.Get("doc"), format: field.Tag.Get("format")}
		if enum := field.Tag.Get("enum"); enum != "" {
			tags.enum = strings.Split(enum, ",")
		}
		schema, err := d.of(field.Type, tags)
		if err != nil {
			return fmt.Errorf("field %s.%s: %w", t, field.Name, err)
		}
		node.properties = append(node.properties, property{name: name, schema: schema})
		if !optional {
			node.required = append(node.required, name)
		}
	}
	return nil
}

// fieldTags are what a field's struct tags say beyond its type.
type fieldTags struct {
	doc, format string
	enum        []string
}

// enumValues reads an enum tag as the values of the field's kind.
func enumValues(kind string, raw []string) ([]any, error) {
	values := make([]any, 0, len(raw))
	for _, value := range raw {
		value = strings.TrimSpace(value)
		switch kind {
		case "string":
			values = append(values, value)
		case "integer":
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("enum value %q is not an integer", value)
			}
			values = append(values, n)
		default:
			return nil, fmt.Errorf("enum is only described for a string or an integer, not %s", kind)
		}
	}
	return values, nil
}

// property is one named property of an object, in the order it is written.
type property struct {
	name   string
	schema *schemaNode
}

// schemaNode is one derived schema. It writes its keywords in a fixed order --
// the kind, its format, its description, its values, then what it contains --
// so a document derived twice is the same bytes.
type schemaNode struct {
	ref         string
	any         bool
	kind        string
	null        bool
	format      string
	description string
	enum        []any
	items       *schemaNode
	object      bool
	required    []string
	properties  []property
	additional  *schemaNode
	anyOf       []*schemaNode
}

// nullable is the node, or null.
func (n *schemaNode) nullable() *schemaNode {
	switch {
	case n.any:
		return n
	case n.ref != "" || n.object || n.kind == "array" || n.additional != nil:
		description := n.description
		inner := *n
		inner.description = ""
		return &schemaNode{anyOf: []*schemaNode{&inner, {kind: "null"}}, description: description}
	default:
		out := *n
		out.null = true
		if len(out.enum) > 0 {
			out.enum = append(append([]any(nil), out.enum...), nil)
		}
		return &out
	}
}

// MarshalJSON writes the node with its keywords in their fixed order.
func (n *schemaNode) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	first := true
	write := func(key string, value any) error {
		if !first {
			b.WriteByte(',')
		}
		first = false
		encodedKey, _ := json.Marshal(key)
		b.Write(encodedKey)
		b.WriteByte(':')
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		b.Write(encoded)
		return nil
	}
	var err error
	switch {
	case n.ref != "":
		err = errors.Join(write("$ref", n.ref), n.describe(write))
	case len(n.anyOf) > 0:
		err = errors.Join(write("anyOf", n.anyOf), n.describe(write))
	case n.any:
		err = n.describe(write)
	default:
		var kind any = n.kind
		if n.null {
			kind = []string{n.kind, "null"}
		}
		err = write("type", kind)
		if err == nil && n.format != "" {
			err = write("format", n.format)
		}
		if err == nil {
			err = n.describe(write)
		}
		if err == nil && len(n.enum) > 0 {
			err = write("enum", n.enum)
		}
		if err == nil && n.items != nil {
			err = write("items", n.items)
		}
		if err == nil && n.object {
			if len(n.required) > 0 {
				err = write("required", n.required)
			}
			if err == nil {
				err = write("properties", orderedProperties(n.properties))
			}
		}
		if err == nil && n.additional != nil {
			err = write("additionalProperties", n.additional)
		}
	}
	if err != nil {
		return nil, err
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// describe writes the description, when there is one.
func (n *schemaNode) describe(write func(string, any) error) error {
	if n.description == "" {
		return nil
	}
	return write("description", n.description)
}

// orderedProperties writes an object's properties in declaration order.
type orderedProperties []property

// MarshalJSON writes the properties as one object, in order.
func (p orderedProperties) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, prop := range p {
		if i > 0 {
			b.WriteByte(',')
		}
		key, _ := json.Marshal(prop.name)
		b.Write(key)
		b.WriteByte(':')
		value, err := prop.schema.MarshalJSON()
		if err != nil {
			return nil, err
		}
		b.Write(value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
