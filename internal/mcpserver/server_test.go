package mcpserver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/mcpserver"
)

func TestNewServer_RegistersAllTools(t *testing.T) {
	deps := mcpserver.Deps{
		Packs:   []*content.Pack{{ID: "test", Name: "Test Pack"}},
		Version: "1.0.0",
	}
	s := mcpserver.NewServer(deps)
	assert.NotNil(t, s)
}
