package schema_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
)

// schemasDir returns the absolute path to content/schemas/ relative to this
// test file's location.
func schemasDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile: .../internal/schema/schema_test.go
	// content/schemas is ../../content/schemas from there
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "content", "schemas")
}

func TestLoad_Resources(t *testing.T) {
	s, err := schema.Load(schemasDir(), "resources")
	require.NoError(t, err)

	assert.Equal(t, "array", s.TopType)
	require.NotNil(t, s.ItemSpec)

	// required fields
	assert.Contains(t, s.ItemSpec.Required, "id")
	assert.Contains(t, s.ItemSpec.Required, "title")
	assert.Contains(t, s.ItemSpec.Required, "url")
	assert.Contains(t, s.ItemSpec.Required, "type")
	assert.Contains(t, s.ItemSpec.Required, "tags")

	// find the "type" field and check its enum values
	var typeField *schema.FieldSpec
	for i := range s.ItemSpec.Fields {
		if s.ItemSpec.Fields[i].Key == "type" {
			typeField = &s.ItemSpec.Fields[i]
			break
		}
	}
	require.NotNil(t, typeField, "type field not found")
	assert.Contains(t, typeField.Enum, "official-docs")
	assert.Contains(t, typeField.Enum, "sample")
	assert.Contains(t, typeField.Enum, "community")
	assert.Contains(t, typeField.Enum, "tutorial")
	assert.Contains(t, typeField.Enum, "blog")

	// required fields sorted first
	assert.True(t, s.ItemSpec.Fields[0].Required, "first field should be required")
}

func TestLoad_Pack(t *testing.T) {
	s, err := schema.Load(schemasDir(), "pack")
	require.NoError(t, err)

	assert.Equal(t, "object", s.TopType)
	require.NotNil(t, s.ObjectSpec)

	assert.Contains(t, s.ObjectSpec.Required, "id")
	assert.Contains(t, s.ObjectSpec.Required, "name")
	assert.Contains(t, s.ObjectSpec.Required, "description")
	assert.Contains(t, s.ObjectSpec.Required, "tags")

	// conditional: when additive==true, additive_position is constrained
	require.NotEmpty(t, s.ObjectSpec.Conditionals, "expected at least one conditional")
	cond := s.ObjectSpec.Conditionals[0]
	assert.Equal(t, "additive", cond.TriggerField)
	assert.Equal(t, "true", cond.TriggerConst)
}

func TestLoad_Tools_NestedObject(t *testing.T) {
	s, err := schema.Load(schemasDir(), "tools")
	require.NoError(t, err)

	assert.Equal(t, "array", s.TopType)
	require.NotNil(t, s.ItemSpec)

	// find detect field
	var detectField *schema.FieldSpec
	for i := range s.ItemSpec.Fields {
		if s.ItemSpec.Fields[i].Key == "detect" {
			detectField = &s.ItemSpec.Fields[i]
			break
		}
	}
	require.NotNil(t, detectField, "detect field not found")
	assert.Equal(t, "object", detectField.Type)
	assert.Len(t, detectField.Children, 2, "detect should have command and pattern children")

	// verify children have expected keys
	childKeys := make([]string, 0, 2)
	for _, c := range detectField.Children {
		childKeys = append(childKeys, c.Key)
	}
	assert.Contains(t, childKeys, "command")
	assert.Contains(t, childKeys, "pattern")
}

func TestLoad_Influencers_MapType(t *testing.T) {
	s, err := schema.Load(schemasDir(), "influencers")
	require.NoError(t, err)

	assert.Equal(t, "array", s.TopType)
	require.NotNil(t, s.ItemSpec)

	var linksField *schema.FieldSpec
	for i := range s.ItemSpec.Fields {
		if s.ItemSpec.Fields[i].Key == "links" {
			linksField = &s.ItemSpec.Fields[i]
			break
		}
	}
	require.NotNil(t, linksField, "links field not found")
	assert.Equal(t, "map", linksField.Type)
	assert.Equal(t, "uri", linksField.Format)
}

func TestLoad_AllSchemas(t *testing.T) {
	names := []string{
		"resources", "pack", "influencers",
		"event-types", "event-instances",
		"mcp", "tools", "hook", "samples", "known_errors",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			s, err := schema.Load(schemasDir(), name)
			require.NoError(t, err)
			assert.NotEmpty(t, s.TopType, "schema %s has no type", name)
		})
	}
}

func TestLoad_NonExistent(t *testing.T) {
	_, err := schema.Load(schemasDir(), "does-not-exist")
	require.Error(t, err)
}

func TestSchemaForFile(t *testing.T) {
	name, ok := schema.SchemaForFile("resources.yaml")
	assert.True(t, ok)
	assert.Equal(t, "resources", name)

	name, ok = schema.SchemaForFile("/path/to/pack.yaml")
	assert.True(t, ok)
	assert.Equal(t, "pack", name)

	_, ok = schema.SchemaForFile("unknown.yaml")
	assert.False(t, ok)
}
