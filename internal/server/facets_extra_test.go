package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDocTypeTagFilters(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		assert.Nil(t, buildDocTypeTagFilters(nil, nil))
		assert.Nil(t, buildDocTypeTagFilters([]string{}, []string{}))
	})

	t.Run("docTypes only", func(t *testing.T) {
		filters := buildDocTypeTagFilters([]string{"invoice", "receipt"}, nil)
		require.Len(t, filters, 1)
		terms, ok := filters[0]["terms"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, []string{"invoice", "receipt"}, terms["docType.keyword"])
	})

	t.Run("tags only", func(t *testing.T) {
		filters := buildDocTypeTagFilters(nil, []string{"utility"})
		require.Len(t, filters, 1)
		terms, ok := filters[0]["terms"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, []string{"utility"}, terms["tags.keyword"])
	})

	t.Run("both produce two clauses", func(t *testing.T) {
		filters := buildDocTypeTagFilters([]string{"tax"}, []string{"2024", "federal"})
		assert.Len(t, filters, 2)
	})
}

func TestDocTypeTagAggs(t *testing.T) {
	aggs := docTypeTagAggs()
	require.Contains(t, aggs, "docTypes")
	require.Contains(t, aggs, "tags")

	docTypes := aggs["docTypes"].(map[string]any)["terms"].(map[string]any)
	assert.Equal(t, "docType.keyword", docTypes["field"])

	tags := aggs["tags"].(map[string]any)["terms"].(map[string]any)
	assert.Equal(t, "tags.keyword", tags["field"])
}
