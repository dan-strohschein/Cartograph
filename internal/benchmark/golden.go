package benchmark

// Scenario defines a benchmark scenario with its query parameters and
// the grep targets an agent would use to answer the same question from source.
type Scenario struct {
	Name      string   // Human-readable scenario name
	QueryType string   // "depends", "field", "effects"
	Target    string   // Query target (e.g., "Bundle", "Bundle.Name")
	Target2   string   // Second target for field queries (field name)
	GrepTerms []string // Terms an agent would grep in source code
}

// Scenarios defines the 10 benchmark scenarios covering three query types
// that work well with L1-extracted AID files (depends, field, effects).
//
// CallStack and ErrorProducers are excluded because the SyndrDB AID files
// lack @calls and @errors-with-type-references, which are L2+ features.
var Scenarios = []Scenario{
	// --- TypeDependents: "What depends on this type?" ---
	{
		Name:      "depends-Bundle",
		QueryType: "depends",
		Target:    "Bundle",
		GrepTerms: []string{"Bundle"},
	},
	{
		Name:      "depends-Document",
		QueryType: "depends",
		Target:    "Document",
		GrepTerms: []string{"Document"},
	},
	{
		Name:      "depends-BundleFieldSchema",
		QueryType: "depends",
		Target:    "BundleFieldSchema",
		GrepTerms: []string{"BundleFieldSchema"},
	},
	{
		Name:      "depends-FieldValue",
		QueryType: "depends",
		Target:    "FieldValue",
		GrepTerms: []string{"FieldValue"},
	},
	{
		Name:      "depends-DocumentCommand",
		QueryType: "depends",
		Target:    "DocumentCommand",
		GrepTerms: []string{"DocumentCommand"},
	},

	// --- FieldTouchers: "What touches this field?" ---
	{
		Name:      "field-Bundle.Name",
		QueryType: "field",
		Target:    "Bundle",
		Target2:   "Name",
		GrepTerms: []string{"Bundle", ".Name"},
	},
	{
		Name:      "field-Document.DocumentID",
		QueryType: "field",
		Target:    "Document",
		Target2:   "DocumentID",
		GrepTerms: []string{"DocumentID"},
	},
	{
		Name:      "field-FieldValue.Type",
		QueryType: "field",
		Target:    "FieldValue",
		Target2:   "Type",
		GrepTerms: []string{"FieldValue", ".Type"},
	},
	{
		Name:      "field-WALConfig.MaxFileSize",
		QueryType: "field",
		Target:    "WALConfig",
		Target2:   "MaxFileSize",
		GrepTerms: []string{"MaxFileSize"},
	},

	// --- SideEffects: "What are the side effects?" ---
	{
		Name:      "effects-AsyncWALAdapter.WriteEntries",
		QueryType: "effects",
		Target:    "AsyncWALAdapter.WriteEntries",
		GrepTerms: []string{"WriteEntries", "AsyncWALAdapter"},
	},
}
