package server

// buildDocTypeTagFilters constructs OpenSearch terms-filter clauses for the
// AI-derived docType and tags fields. It mirrors how buildSearchFilters uses
// the ".keyword" sub-field for exact-match faceting (see company.name.keyword).
// Returns nil when neither docTypes nor tags are provided. The backend wiring
// engineer composes the returned clauses into the existing filter slice in the
// search/facets handlers.
func buildDocTypeTagFilters(docTypes []string, tags []string) []map[string]any {
	var filters []map[string]any

	if len(docTypes) > 0 {
		filters = append(filters, map[string]any{
			"terms": map[string]any{
				"docType.keyword": docTypes,
			},
		})
	}

	if len(tags) > 0 {
		filters = append(filters, map[string]any{
			"terms": map[string]any{
				"tags.keyword": tags,
			},
		})
	}

	return filters
}

// docTypeTagAggs returns the terms aggregations for the AI-derived docType and
// tags facets. These are merged into the existing aggs map in
// handleSearchFacets by the backend wiring engineer.
func docTypeTagAggs() map[string]any {
	return map[string]any{
		"docTypes": map[string]any{
			"terms": map[string]any{
				"field": "docType.keyword",
				"size":  20,
			},
		},
		"tags": map[string]any{
			"terms": map[string]any{
				"field": "tags.keyword",
				"size":  30,
			},
		},
	}
}
