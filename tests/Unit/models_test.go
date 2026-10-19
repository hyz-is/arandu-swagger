package unit_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	fhttp "github.com/arandu-io/framework/http"
	swagger "github.com/hyz-is/arandu-swagger"
)

type modelCard struct {
	Brand    string  `json:"brand"`
	Last4    *string `json:"last4"`
	ExpMonth *int    `json:"exp_month"`
}

type modelPaymentMethod struct {
	ID        string    `json:"id"`
	Type      string    `json:"type" enum:"card"`
	Card      modelCard `json:"card"`
	Amount    int64     `json:"amount_cents" doc:"Valor em centavos."`
	URL       string    `json:"url" format:"uri" doc:"Para onde o comprador vai."`
	PaidAt    *string   `json:"paid_at" format:"date-time"`
	CreatedAt time.Time `json:"created_at"`
	secret    string
	Skipped   string `json:"-"`
}

type modelError struct {
	Error   string              `json:"error" doc:"Código do erro."`
	Message string              `json:"message,omitempty"`
	Fields  map[string][]string `json:"fields,omitempty"`
}

type modelData[T any] struct {
	Data T `json:"data"`
}

type modelPage[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

type modelAudit struct {
	By string `json:"by"`
}

type modelEmbedding struct {
	modelAudit
	ID    string         `json:"id"`
	Note  *modelCard     `json:"note,omitzero"`
	Extra map[string]any `json:"extra,omitempty"`
	Raw   json.RawMessage
}

func TestSchemasFromModelsWriteTheTypesAsEncodingJSONWritesThem(t *testing.T) {
	t.Parallel()

	schemas, err := swagger.SchemasFromModels(
		swagger.ModelOf[modelCard]("Card"),
		swagger.ModelOf[modelPaymentMethod]("PaymentMethod"),
		swagger.ModelOf[modelError]("Error"),
		swagger.ModelOf[modelData[modelPaymentMethod]]("PaymentMethodEnvelope"),
		swagger.ModelOf[modelPage[modelPaymentMethod]]("PaymentMethodPage"),
		swagger.ModelOf[modelData[modelPage[modelPaymentMethod]]]("PaymentMethodPageEnvelope"),
		swagger.ModelOf[modelEmbedding]("Embedding"),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"Card": `{"type":"object","required":["brand","last4","exp_month"],"properties":{"brand":{"type":"string"},"last4":{"type":["string","null"]},"exp_month":{"type":["integer","null"]}}}`,
		"PaymentMethod": `{"type":"object","required":["id","type","card","amount_cents","url","paid_at","created_at"],"properties":{` +
			`"id":{"type":"string"},"type":{"type":"string","enum":["card"]},"card":{"$ref":"#/components/schemas/Card"},` +
			`"amount_cents":{"type":"integer","description":"Valor em centavos."},` +
			`"url":{"type":"string","format":"uri","description":"Para onde o comprador vai."},` +
			`"paid_at":{"type":["string","null"],"format":"date-time"},"created_at":{"type":"string","format":"date-time"}}}`,
		"Error": `{"type":"object","required":["error"],"properties":{"error":{"type":"string","description":"Código do erro."},` +
			`"message":{"type":"string"},"fields":{"type":"object","additionalProperties":{"type":"array","items":{"type":"string"}}}}}`,
		"PaymentMethodEnvelope":     `{"type":"object","required":["data"],"properties":{"data":{"$ref":"#/components/schemas/PaymentMethod"}}}`,
		"PaymentMethodPage":         `{"type":"object","required":["items","total"],"properties":{"items":{"type":"array","items":{"$ref":"#/components/schemas/PaymentMethod"}},"total":{"type":"integer"}}}`,
		"PaymentMethodPageEnvelope": `{"type":"object","required":["data"],"properties":{"data":{"$ref":"#/components/schemas/PaymentMethodPage"}}}`,
		"Embedding": `{"type":"object","required":["by","id","Raw"],"properties":{"by":{"type":"string"},"id":{"type":"string"},` +
			`"note":{"anyOf":[{"$ref":"#/components/schemas/Card"},{"type":"null"}]},"extra":{"type":"object","additionalProperties":{}},"Raw":{}}}`,
	}
	if len(schemas) != len(want) {
		t.Fatalf("derived %d schemas, want %d", len(schemas), len(want))
	}
	for name, expected := range want {
		got, err := json.Marshal(schemas[name])
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(got) != expected {
			t.Errorf("%s:\n got %s\nwant %s", name, got, expected)
		}
	}
}

func TestSchemasFromModelsRefuseWhatTheyCannotSay(t *testing.T) {
	t.Parallel()

	type withChannel struct {
		C chan int `json:"c"`
	}
	type withString struct {
		N int `json:"n,string"`
	}
	type withIntKeys struct {
		M map[int]string `json:"m"`
	}
	type withBytes struct {
		B []byte `json:"b"`
	}
	type withBadEnum struct {
		N int `json:"n" enum:"one"`
	}
	for name, model := range map[string]swagger.Model{
		"a channel":                      swagger.ModelOf[withChannel]("C"),
		"the string option":              swagger.ModelOf[withString]("S"),
		"a map with integer keys":        swagger.ModelOf[withIntKeys]("M"),
		"bytes":                          swagger.ModelOf[withBytes]("B"),
		"an enum of the wrong kind":      swagger.ModelOf[withBadEnum]("E"),
		"a name a component cannot take": swagger.ModelOf[modelCard]("not a name"),
		"no type":                        {Name: "Nothing"},
	} {
		if _, err := swagger.SchemasFromModels(model); err == nil {
			t.Errorf("%s: derived a schema, want an error", name)
		}
	}
	if _, err := swagger.SchemasFromModels(swagger.ModelOf[modelCard]("Card"), swagger.ModelOf[modelCard]("Other")); err == nil {
		t.Error("two names for one type: derived, want an error")
	}
}

func TestRegistryModelsAreComponentsTheGeneratedDocumentCarries(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	route := router.Get("/api/methods", func(http.ResponseWriter, *http.Request) {}).Name("methods.index")
	registry := swagger.NewRegistry()
	if err := registry.Models(swagger.ModelOf[modelCard]("Card"), swagger.ModelOf[modelPaymentMethod]("PaymentMethod")); err != nil {
		t.Fatal(err)
	}
	registry.Route(route).Summary("List methods").Response(http.StatusOK, "The methods.", swagger.JSONRef("PaymentMethod"))
	doc, err := swagger.Generate(router.Routes(), registry, swagger.Config{Title: "Test", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"Card":{"type":"object"`) || !strings.Contains(string(encoded), `"$ref":"#/components/schemas/Card"`) {
		t.Fatalf("generated document misses the derived components: %s", encoded)
	}

	type unnamed struct {
		Card modelCard `json:"card"`
	}
	failing := swagger.NewRegistry()
	if err := failing.Models(swagger.ModelOf[unnamed]("Holder")); err != nil {
		t.Fatalf("an unnamed reference is inlined, not refused: %v", err)
	}
	if err := failing.Models(swagger.ModelOf[modelCard]("Card"), swagger.ModelOf[modelCard]("Twice")); err == nil {
		t.Fatal("Models registered two names for one type")
	}
}
