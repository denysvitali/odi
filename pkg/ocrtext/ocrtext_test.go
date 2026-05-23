package ocrtext

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/denysvitali/odi/pkg/ocrclient"
)

func mkBlock(text string, left, top, right, bottom int) ocrclient.TextBlock {
	return ocrclient.TextBlock{
		Text: text,
		BoundingBox: ocrclient.BoundingBox{
			Left:   left,
			Top:    top,
			Right:  right,
			Bottom: bottom,
		},
	}
}

func TestGetText_Nil(t *testing.T) {
	assert.Equal(t, "", GetText(nil, DefaultMergeDistance, DefaultHorizontalDistance))
}

func TestGetText_Empty(t *testing.T) {
	out := GetText(&ocrclient.OcrResult{}, DefaultMergeDistance, DefaultHorizontalDistance)
	assert.Equal(t, "", out)
}

func TestGetText_SingleBlock(t *testing.T) {
	res := &ocrclient.OcrResult{
		TextBlocks: []ocrclient.TextBlock{
			mkBlock("Hello world", 10, 10, 200, 30),
		},
	}
	out := GetText(res, DefaultMergeDistance, DefaultHorizontalDistance)
	assert.Contains(t, out, "Hello world")
}

func TestGetText_VerticalOrderingInSameColumn(t *testing.T) {
	// Two blocks in roughly the same column (left within mergeDistance)
	// with significantly different top values → should appear top-first
	// separated by a blank line.
	res := &ocrclient.OcrResult{
		TextBlocks: []ocrclient.TextBlock{
			mkBlock("Bottom", 20, 500, 200, 530),
			mkBlock("Top", 20, 10, 200, 40),
		},
	}
	out := GetText(res, DefaultMergeDistance, DefaultHorizontalDistance)
	topIdx := strings.Index(out, "Top")
	bottomIdx := strings.Index(out, "Bottom")
	assert.GreaterOrEqual(t, topIdx, 0)
	assert.Greater(t, bottomIdx, topIdx, "Top should appear before Bottom")
	// They should be separated by a blank line because the vertical gap is large
	assert.Contains(t, out, "\n\n")
}

func TestGetText_TwoColumns(t *testing.T) {
	// Left column near x=0, right column near x=1000 — should land in
	// different buckets given mergeDistance=150.
	res := &ocrclient.OcrResult{
		TextBlocks: []ocrclient.TextBlock{
			mkBlock("LeftA", 10, 100, 100, 130),
			mkBlock("RightA", 1000, 100, 1100, 130),
			mkBlock("LeftB", 10, 200, 100, 230),
			mkBlock("RightB", 1000, 200, 1100, 230),
		},
	}
	out := GetText(res, DefaultMergeDistance, DefaultHorizontalDistance)
	// Both columns must appear in the output
	assert.Contains(t, out, "LeftA")
	assert.Contains(t, out, "LeftB")
	assert.Contains(t, out, "RightA")
	assert.Contains(t, out, "RightB")
	// Within a column, top-most must come first
	assert.Less(t, strings.Index(out, "LeftA"), strings.Index(out, "LeftB"))
	assert.Less(t, strings.Index(out, "RightA"), strings.Index(out, "RightB"))
}

func TestGetText_BucketSizeFallback(t *testing.T) {
	// mergeDistance <= 0 must not crash and must still produce output.
	res := &ocrclient.OcrResult{
		TextBlocks: []ocrclient.TextBlock{
			mkBlock("One", 0, 0, 100, 20),
			mkBlock("Two", 5, 0, 100, 20),
		},
	}
	out := GetText(res, 0, DefaultHorizontalDistance)
	assert.Contains(t, out, "One")
	assert.Contains(t, out, "Two")

	out2 := GetText(res, -42, DefaultHorizontalDistance)
	assert.Contains(t, out2, "One")
	assert.Contains(t, out2, "Two")
}

func TestGetText_InlineBlocksSeparatedBySpace(t *testing.T) {
	// Two blocks at the same top within same column → joined with a single space.
	res := &ocrclient.OcrResult{
		TextBlocks: []ocrclient.TextBlock{
			mkBlock("Foo", 10, 100, 50, 120),
			mkBlock("Bar", 60, 100, 100, 120),
		},
	}
	out := GetText(res, DefaultMergeDistance, DefaultHorizontalDistance)
	assert.Contains(t, out, "Foo Bar")
}
