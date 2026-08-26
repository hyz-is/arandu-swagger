package swagger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/arandu-io/hesape/jsonschema"
)

const schemaComponentPrefix = "#/components/schemas/"

var (
	singleSubschemaKeywords = []string{
		"additionalItems",
		"additionalProperties",
		"contains",
		"contentSchema",
		"else",
		"if",
		"items",
		"not",
		"propertyNames",
		"then",
		"unevaluatedItems",
		"unevaluatedProperties",
	}
	arraySubschemaKeywords = []string{
		"allOf",
		"anyOf",
		"oneOf",
		"prefixItems",
	}
	mapSubschemaKeywords = []string{
		"$defs",
		"definitions",
		"dependentSchemas",
		"patternProperties",
		"properties",
	}
)

// Schema is an immutable OpenAPI schema snapshot or a schema reference.
// Native schema builders are snapshotted by SchemaFrom so later builder
// mutations cannot race with or alter document generation.
type Schema struct {
	json      json.RawMessage
	reference string
}

// SchemaFrom snapshots a native Hesape JSON Schema type for use in OpenAPI.
// It returns an error when the type is nil, panics while marshaling, or does
// not produce a structurally valid JSON Schema object supported by this
// adapter.
func SchemaFrom(native jsonschema.Type) (schema Schema, err error) {
	if isNilSchema(native) {
		return Schema{}, fmt.Errorf("swagger: cannot snapshot a nil schema")
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			schema = Schema{}
			err = fmt.Errorf("swagger: schema marshal panic: %v", recovered)
		}
	}()

	encoded, err := json.Marshal(native)
	if err != nil {
		return Schema{}, fmt.Errorf("swagger: marshal schema: %w", err)
	}
	snapshot, err := schemaFromJSON(encoded)
	if err != nil {
		return Schema{}, err
	}
	if err := validateSchemaSnapshot(snapshot.json); err != nil {
		return Schema{}, fmt.Errorf("swagger: invalid native schema: %w", err)
	}
	return snapshot, nil
}

// SchemaRef returns a local reference to a named schema component.
func SchemaRef(name string) Schema {
	return Schema{reference: schemaComponentPrefix + escapeJSONPointerToken(name)}
}

// SchemaReference returns a schema that uses the exact reference supplied by
// the caller. Its URI-reference syntax is checked when marshaled and during
// document generation.
func SchemaReference(reference string) Schema {
	return Schema{reference: reference}
}

// IsZero reports whether the schema contains neither a snapshot nor a
// reference.
func (s Schema) IsZero() bool {
	return len(s.json) == 0 && s.reference == ""
}

// IsReference reports whether the schema is represented by a reference.
func (s Schema) IsReference() bool {
	return s.reference != ""
}

// Reference returns the schema reference, or an empty string for an inline
// schema snapshot.
func (s Schema) Reference() string {
	return s.reference
}

// JSON returns a copy of an inline schema snapshot. It returns nil for a
// reference or an empty schema.
func (s Schema) JSON() json.RawMessage {
	if len(s.json) == 0 {
		return nil
	}
	return bytes.Clone(s.json)
}

// MarshalJSON renders the immutable schema snapshot or reference.
func (s Schema) MarshalJSON() ([]byte, error) {
	switch {
	case len(s.json) > 0 && s.reference != "":
		return nil, fmt.Errorf("swagger: schema cannot contain both inline JSON and a reference")
	case len(s.json) > 0:
		return bytes.Clone(s.json), nil
	case s.reference != "":
		if err := validateURIReference(s.reference); err != nil {
			return nil, fmt.Errorf("swagger: invalid schema URI reference %q: %w", s.reference, err)
		}
		return json.Marshal(struct {
			Ref string `json:"$ref"`
		}{Ref: s.reference})
	default:
		return nil, fmt.Errorf("swagger: cannot marshal an empty schema")
	}
}

func schemaFromJSON(encoded []byte) (Schema, error) {
	trimmed := bytes.TrimSpace(encoded)
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' || !json.Valid(trimmed) {
		return Schema{}, fmt.Errorf("swagger: native schema must marshal as a JSON object")
	}
	if err := rejectDuplicateJSONKeys(trimmed); err != nil {
		return Schema{}, err
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return Schema{}, fmt.Errorf("swagger: decode native schema object: %w", err)
	}
	if object == nil {
		return Schema{}, fmt.Errorf("swagger: native schema must marshal as a JSON object")
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return Schema{}, fmt.Errorf("swagger: compact native schema: %w", err)
	}
	return Schema{json: bytes.Clone(compact.Bytes())}, nil
}

func validateSchemaSnapshot(encoded []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("decode schema: %w", err)
	}
	return validateSchemaValue(document, "$")
}

func validateSchemaValue(value any, location string) error {
	switch typed := value.(type) {
	case bool:
		return nil
	case map[string]any:
		if err := validateSchemaKeywords(typed, location); err != nil {
			return err
		}
		for _, keyword := range singleSubschemaKeywords {
			child, exists := typed[keyword]
			if !exists {
				continue
			}
			if err := validateSchemaValue(child, location+"."+keyword); err != nil {
				return err
			}
		}
		for _, keyword := range arraySubschemaKeywords {
			child, exists := typed[keyword]
			if !exists {
				continue
			}
			items, ok := child.([]any)
			if !ok {
				return fmt.Errorf("%s.%s must be an array of schemas", location, keyword)
			}
			for index, item := range items {
				if err := validateSchemaValue(item, fmt.Sprintf("%s.%s[%d]", location, keyword, index)); err != nil {
					return err
				}
			}
		}
		for _, keyword := range mapSubschemaKeywords {
			child, exists := typed[keyword]
			if !exists {
				continue
			}
			items, ok := child.(map[string]any)
			if !ok {
				return fmt.Errorf("%s.%s must be an object containing schemas", location, keyword)
			}
			for name, item := range items {
				if err := validateSchemaValue(item, location+"."+keyword+"."+name); err != nil {
					return err
				}
			}
		}
		if child, exists := typed["dependencies"]; exists {
			dependencies, ok := child.(map[string]any)
			if !ok {
				return fmt.Errorf("%s.dependencies must be an object", location)
			}
			for name, dependency := range dependencies {
				switch dependency.(type) {
				case bool, map[string]any:
					if err := validateSchemaValue(dependency, location+".dependencies."+name); err != nil {
						return err
					}
				case []any:
					// Draft 7 property dependencies are arrays of property names,
					// not nested schemas.
				default:
					return fmt.Errorf("%s.dependencies.%s must be a schema or an array of property names", location, name)
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("%s must be a schema object or boolean", location)
	}
}

func validateSchemaKeywords(object map[string]any, location string) error {
	for _, keyword := range []string{"minLength", "maxLength", "minItems", "maxItems", "minProperties", "maxProperties", "minContains", "maxContains"} {
		if value, exists := object[keyword]; exists {
			if err := validateNonNegativeInteger(value); err != nil {
				return fmt.Errorf("%s.%s must be a non-negative integer", location, keyword)
			}
		}
	}
	if value, exists := object["multipleOf"]; exists {
		number, ok := schemaNumber(value)
		if !ok || number <= 0 {
			return fmt.Errorf("%s.multipleOf must be greater than zero", location)
		}
	}
	if value, exists := object["pattern"]; exists {
		pattern, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s.pattern must be a string", location)
		}
		if err := validatePortablePattern(pattern); err != nil {
			return fmt.Errorf("%s.pattern: %w", location, err)
		}
	}
	if value, exists := object["enum"]; exists {
		values, ok := value.([]any)
		if !ok || len(values) == 0 {
			return fmt.Errorf("%s.enum must be a non-empty array", location)
		}
		if duplicateJSONValue(values) {
			return fmt.Errorf("%s.enum must contain unique values", location)
		}
	}
	if value, exists := object["required"]; exists {
		values, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s.required must be an array", location)
		}
		seen := make(map[string]bool, len(values))
		for _, value := range values {
			name, ok := value.(string)
			if !ok || name == "" {
				return fmt.Errorf("%s.required must contain non-empty property names", location)
			}
			if seen[name] {
				return fmt.Errorf("%s.required contains duplicate property %q", location, name)
			}
			seen[name] = true
		}
	}
	if value, exists := object["type"]; exists {
		if err := validateSchemaTypes(value); err != nil {
			return fmt.Errorf("%s.type: %w", location, err)
		}
	}
	if value, exists := object["$ref"]; exists {
		reference, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s.$ref must be a string", location)
		}
		if err := validateURIReference(reference); err != nil {
			return fmt.Errorf("%s.$ref: %w", location, err)
		}
	}
	return nil
}

func validateNonNegativeInteger(value any) error {
	number, ok := value.(json.Number)
	if !ok {
		return errors.New("not a number")
	}
	integer, err := strconv.ParseInt(number.String(), 10, 64)
	if err != nil || integer < 0 {
		return errors.New("not a non-negative integer")
	}
	return nil
}

func schemaNumber(value any) (float64, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := number.Float64()
	return parsed, err == nil
}

func duplicateJSONValue(values []any) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			return true
		}
		key := string(encoded)
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	return false
}

func validateSchemaTypes(value any) error {
	valid := map[string]bool{
		"null": true, "boolean": true, "object": true, "array": true,
		"number": true, "string": true, "integer": true,
	}
	switch typed := value.(type) {
	case string:
		if !valid[typed] {
			return fmt.Errorf("unsupported JSON Schema type %q", typed)
		}
	case []any:
		if len(typed) == 0 {
			return errors.New("type array must not be empty")
		}
		seen := make(map[string]bool, len(typed))
		for _, item := range typed {
			name, ok := item.(string)
			if !ok || !valid[name] {
				return fmt.Errorf("unsupported JSON Schema type %q", item)
			}
			if seen[name] {
				return fmt.Errorf("type array contains duplicate %q", name)
			}
			seen[name] = true
		}
	default:
		return errors.New("must be a string or an array of strings")
	}
	return nil
}

func rejectDuplicateJSONKeys(encoded []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if err := consumeJSONValue(decoder, "$"); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return errors.New("swagger: native schema contains trailing JSON values")
		}
		return fmt.Errorf("swagger: decode native schema: %w", err)
	}
	return nil
}

func consumeJSONValue(decoder *json.Decoder, location string) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("swagger: decode native schema at %s: %w", location, err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]bool)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("swagger: decode native schema at %s: %w", location, err)
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("swagger: native schema at %s contains a non-string object key", location)
			}
			if seen[key] {
				return fmt.Errorf("swagger: native schema at %s contains duplicate key %q", location, key)
			}
			seen[key] = true
			if err := consumeJSONValue(decoder, location+"."+key); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return fmt.Errorf("swagger: decode native schema at %s: %w", location, err)
		}
	case '[':
		for index := 0; decoder.More(); index++ {
			if err := consumeJSONValue(decoder, fmt.Sprintf("%s[%d]", location, index)); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return fmt.Errorf("swagger: decode native schema at %s: %w", location, err)
		}
	default:
		return fmt.Errorf("swagger: unexpected JSON delimiter %q at %s", delimiter, location)
	}
	return nil
}

func isNilSchema(native jsonschema.Type) bool {
	if native == nil {
		return true
	}
	value := reflect.ValueOf(native)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func escapeJSONPointerToken(value string) string {
	value = strings.ReplaceAll(value, "~", "~0")
	return strings.ReplaceAll(value, "/", "~1")
}
