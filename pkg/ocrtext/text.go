package ocrtext

import (
	"math"
	"sort"
	"strings"

	"github.com/denysvitali/odi/pkg/ocrclient"
)

const (
	// DefaultMergeDistance is the default maximum pixel distance to merge blocks into the same column
	DefaultMergeDistance = 150
	// DefaultHorizontalDistance is the default maximum vertical pixel distance for inline blocks
	DefaultHorizontalDistance = 10
)

// GetText returns the text from the OCR result
// sorted in a way that matches the document text
// order
func GetText(v *ocrclient.OcrResult, mergeDistance float64, horizontalDistance float64) string {
	if v == nil {
		return ""
	}
	columns := map[int][]ocrclient.TextBlock{}
	bucketSize := int(mergeDistance)
	if bucketSize <= 0 {
		bucketSize = 1
	}
	for _, b := range v.TextBlocks {
		left := b.BoundingBox.Left
		bucket := (left / bucketSize) * bucketSize
		columns[bucket] = append(columns[bucket], b)
	}

	for _, c := range columns {
		// Sort by top
		sort.Slice(c, func(i, j int) bool {
			// If the top diff is less than 5, sort by left
			if math.Abs(float64(c[i].BoundingBox.Top-c[j].BoundingBox.Top)) <= horizontalDistance {
				return c[i].BoundingBox.Left < c[j].BoundingBox.Left
			}

			return c[i].BoundingBox.Top < c[j].BoundingBox.Top
		})
	}

	columnsValue := make([][]ocrclient.TextBlock, 0, len(columns))
	for _, v := range columns {
		columnsValue = append(columnsValue, v)
	}

	sort.Slice(columnsValue, func(i, j int) bool {
		if len(columnsValue[i]) == 0 || len(columnsValue[j]) == 0 {
			return false
		}
		if math.Abs(float64(columnsValue[i][0].BoundingBox.Top-columnsValue[j][0].BoundingBox.Top)) <= horizontalDistance {
			return columnsValue[i][0].BoundingBox.Left < columnsValue[j][0].BoundingBox.Left
		}
		return columnsValue[i][0].BoundingBox.Top < columnsValue[j][0].BoundingBox.Top
	})

	var output strings.Builder

	// Print the text
	for _, c := range columnsValue {
		var prevBlock *ocrclient.TextBlock = nil
		for _, b := range c {
			currentBlock := b
			if prevBlock != nil {
				if math.Abs(float64(prevBlock.BoundingBox.Top-b.BoundingBox.Top)) >= horizontalDistance {
					output.WriteString("\n\n")
				} else {
					output.WriteString(" ")
				}
			}
			output.WriteString(b.Text)
			prevBlock = &currentBlock
		}
		output.WriteString("\n")
	}

	return output.String()
}
