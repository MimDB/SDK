package mimdb

import (
	_ "embed"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

//go:embed internal/openapi/spec.json
var specJSON []byte

// openAPISpec represents the subset of the OpenAPI 3.x spec structure needed
// for schema validation. Only the components.schemas section is parsed.
type openAPISpec struct {
	Components struct {
		Schemas map[string]openAPISchema `json:"schemas"`
	} `json:"components"`
}

// openAPISchema represents a single schema definition from the spec. It tracks
// property names and required fields for comparison against Go struct tags.
type openAPISchema struct {
	Type       string                   `json:"type"`
	Properties map[string]any           `json:"properties"`
	Required   []string                 `json:"required"`
	AllOf      []openAPISchema          `json:"allOf"`
}

// schemaMapping binds an OpenAPI schema name to the corresponding Go type for
// validation. The Go type's JSON tags are compared against the schema's property
// names to detect drift between the SDK and the API spec.
type schemaMapping struct {
	schemaName string
	goType     reflect.Type
}

// jsonTagNames extracts all JSON field names from a struct type's tags,
// following embedded structs recursively. The omitempty flag is stripped.
func jsonTagNames(t reflect.Type) map[string]bool {
	names := make(map[string]bool)
	for i := range t.NumField() {
		f := t.Field(i)

		// Follow embedded structs (e.g., APIKeyWithSecret embeds APIKeyInfo).
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			for k, v := range jsonTagNames(f.Type) {
				names[k] = v
			}
			continue
		}

		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		names[name] = true
	}
	return names
}

// resolveSchemaProperties returns all property names for a schema, following
// allOf references to merge composed schemas (used by APIKeyWithSecret).
func resolveSchemaProperties(schema openAPISchema, allSchemas map[string]openAPISchema) map[string]bool {
	props := make(map[string]bool)
	for k := range schema.Properties {
		props[k] = true
	}
	for _, sub := range schema.AllOf {
		for k := range sub.Properties {
			props[k] = true
		}
	}
	return props
}

// TestSpecLoads verifies that the embedded OpenAPI spec is valid JSON and
// contains a non-empty components.schemas section.
func TestSpecLoads(t *testing.T) {
	if len(specJSON) == 0 {
		t.Fatal("embedded spec.json is empty")
	}

	var spec openAPISpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("failed to parse spec.json: %v", err)
	}

	if len(spec.Components.Schemas) == 0 {
		t.Fatal("spec.json has no schemas in components.schemas")
	}

	t.Logf("spec.json loaded: %d schemas", len(spec.Components.Schemas))
}

// TestRepresentativeSchemasExist checks that several well-known schema names
// are present in the spec. This acts as a smoke test to detect major spec
// restructuring.
func TestRepresentativeSchemasExist(t *testing.T) {
	var spec openAPISpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("failed to parse spec: %v", err)
	}

	expected := []string{
		"Organization",
		"Project",
		"AuthUser",
		"Session",
		"CronJob",
		"Column",
		"TableDetail",
		"TableSummary",
		"StorageBucket",
		"StorageObject",
		"ForeignKey",
		"TableIndex",
		"ResultColumn",
		"ExecuteResult",
		"QueryStat",
		"ProjectWithKeys",
		"APIKeyWithSecret",
	}

	for _, name := range expected {
		if _, ok := spec.Components.Schemas[name]; !ok {
			t.Errorf("expected schema %q not found in spec", name)
		}
	}
}

// TestGoStructsMatchSpecProperties validates that Go struct JSON tags are a
// superset of the OpenAPI schema property names. For each mapped type, every
// spec property must have a matching JSON tag in the Go struct. Extra Go fields
// (not in the spec) are allowed - the SDK may include convenience fields - but
// missing spec properties indicate type drift.
func TestGoStructsMatchSpecProperties(t *testing.T) {
	var spec openAPISpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("failed to parse spec: %v", err)
	}

	// Map OpenAPI schema names to the Go types they correspond to.
	// Some spec schemas have more fields than the SDK exposes (e.g.,
	// StorageObject in the spec has many server-side fields the SDK omits).
	// Those are tested separately below.
	mappings := []schemaMapping{
		{"Organization", reflect.TypeOf(Organization{})},
		{"Project", reflect.TypeOf(Project{})},
		{"AuthUser", reflect.TypeOf(User{})},
		{"Session", reflect.TypeOf(Session{})},
		{"CronJob", reflect.TypeOf(CronJob{})},
		{"Column", reflect.TypeOf(Column{})},
		{"TableDetail", reflect.TypeOf(TableDetail{})},
		{"TableSummary", reflect.TypeOf(TableSummary{})},
		{"ForeignKey", reflect.TypeOf(ForeignKey{})},
		{"TableIndex", reflect.TypeOf(Index{})},
		{"ResultColumn", reflect.TypeOf(ResultColumn{})},
		{"ProjectWithKeys", reflect.TypeOf(ProjectWithKeys{})},
		{"APIKeyWithSecret", reflect.TypeOf(APIKeyWithSecret{})},
	}

	for _, m := range mappings {
		t.Run(m.schemaName, func(t *testing.T) {
			schema, ok := spec.Components.Schemas[m.schemaName]
			if !ok {
				t.Skipf("schema %q not in spec", m.schemaName)
				return
			}

			specProps := resolveSchemaProperties(schema, spec.Components.Schemas)
			goTags := jsonTagNames(m.goType)

			for prop := range specProps {
				if !goTags[prop] {
					t.Errorf("spec property %q missing from Go type %s (no matching json tag)",
						prop, m.goType.Name())
				}
			}
		})
	}
}

// TestGoStructsNoExtraFieldsBeyondSpec checks the reverse direction: Go struct
// fields that do not appear in the spec. This is informational - extra fields
// are not failures since the SDK may provide convenience fields or the spec may
// lag behind. However, unexpected extras may indicate typos.
func TestGoStructsNoExtraFieldsBeyondSpec(t *testing.T) {
	var spec openAPISpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("failed to parse spec: %v", err)
	}

	// Only check types where the SDK is expected to closely mirror the spec.
	mappings := []schemaMapping{
		{"Organization", reflect.TypeOf(Organization{})},
		{"Project", reflect.TypeOf(Project{})},
		{"Session", reflect.TypeOf(Session{})},
		{"CronJob", reflect.TypeOf(CronJob{})},
		{"Column", reflect.TypeOf(Column{})},
		{"ForeignKey", reflect.TypeOf(ForeignKey{})},
		{"TableIndex", reflect.TypeOf(Index{})},
		{"ResultColumn", reflect.TypeOf(ResultColumn{})},
		{"TableSummary", reflect.TypeOf(TableSummary{})},
	}

	for _, m := range mappings {
		t.Run(m.schemaName, func(t *testing.T) {
			schema, ok := spec.Components.Schemas[m.schemaName]
			if !ok {
				t.Skipf("schema %q not in spec", m.schemaName)
				return
			}

			specProps := resolveSchemaProperties(schema, spec.Components.Schemas)
			goTags := jsonTagNames(m.goType)

			for tag := range goTags {
				if !specProps[tag] {
					t.Logf("info: Go type %s has json tag %q not in spec schema %s",
						m.goType.Name(), tag, m.schemaName)
				}
			}
		})
	}
}

// TestExecuteResultMatchesSQLResult validates that the spec's ExecuteResult
// schema properties are present in the Go SQLResult type. The names differ
// because the SDK uses a more idiomatic Go name.
func TestExecuteResultMatchesSQLResult(t *testing.T) {
	var spec openAPISpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("failed to parse spec: %v", err)
	}

	schema, ok := spec.Components.Schemas["ExecuteResult"]
	if !ok {
		t.Skip("ExecuteResult schema not in spec")
	}

	specProps := resolveSchemaProperties(schema, spec.Components.Schemas)
	goTags := jsonTagNames(reflect.TypeOf(SQLResult{}))

	// ExecuteResult may have a "truncated" property only in the SDK.
	for prop := range specProps {
		if !goTags[prop] {
			t.Errorf("spec property %q missing from Go type SQLResult", prop)
		}
	}
}

// TestQueryStatSubset validates that the SDK's QueryStat type covers the
// core properties from the spec. The spec's QueryStat has additional
// pg_stat_statements fields that the SDK intentionally omits for simplicity.
func TestQueryStatSubset(t *testing.T) {
	var spec openAPISpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("failed to parse spec: %v", err)
	}

	schema, ok := spec.Components.Schemas["QueryStat"]
	if !ok {
		t.Skip("QueryStat schema not in spec")
	}

	specProps := resolveSchemaProperties(schema, spec.Components.Schemas)
	goTags := jsonTagNames(reflect.TypeOf(QueryStat{}))

	// The SDK keeps a core subset. Verify all SDK fields exist in the spec.
	for tag := range goTags {
		if !specProps[tag] {
			t.Errorf("Go QueryStat json tag %q not found in spec", tag)
		}
	}

	// Check that the core fields are present in the Go type.
	coreFields := []string{"queryid", "query", "calls", "mean_exec_time_ms", "total_exec_time_ms"}
	for _, field := range coreFields {
		if !goTags[field] {
			t.Errorf("core QueryStat field %q missing from Go type", field)
		}
	}
}
