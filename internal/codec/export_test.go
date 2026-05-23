package codec

// Test-only exports. Keeping these in an export_test.go file (rather than
// promoting them to the public API) limits the blast radius — anything
// imported from here is only visible to tests in this package.

var (
	AnyToIR            = anyToIR
	IRToAny            = irToAny
	ScalarToAny        = scalarToAny
	CtyToIR            = ctyToIR
	IRToCty            = irToCty
	ScalarToCty        = scalarToCty
	YamlNodeToIR       = yamlNodeToIR
	IRToYAMLNode       = irToYAMLNode
	YAMLTagToScalarTag = yamlTagToScalarTag
)
