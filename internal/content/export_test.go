package content

// Test-only exports — compiled only during go test; not part of the production binary.
func UnionStrings(a, b []string) []string                          { return unionStrings(a, b) }
func MergeResources(base, add []Resource, id string) []Resource    { return mergeResources(base, add, id) }
func MergeTools(base, add []ToolDef) []ToolDef                     { return mergeTools(base, add) }
func MergeMCPServers(base, add []MCPServer, id string) []MCPServer { return mergeMCPServers(base, add, id) }
func MergeHooks(base, add []HookDef, id string) []HookDef         { return mergeHooks(base, add, id) }
func MergeSamples(base, add []Sample, id string) []Sample          { return mergeSamples(base, add, id) }
func RenderDynamic(d *DynamicContext) string                        { return renderDynamic(d) }
