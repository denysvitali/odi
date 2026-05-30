package models

import (
	"time"

	"github.com/denysvitali/odi/zefix-tools/pkg/zefix"
)

type Document struct {
	Date               *time.Time      `json:"date,omitempty"`
	Title              string          `json:"title,omitempty"`
	Text               string          `json:"text,omitempty"`
	Barcode            *Barcode        `json:"barcode,omitempty"`
	AdditionalBarcodes []Barcode       `json:"additionalBarcodes"`
	Company            *zefix.Company  `json:"company,omitempty"`
	Companies          []zefix.Company `json:"companies,omitempty"`
	Dates              []time.Time     `json:"dates,omitempty"`
	IndexedAt          time.Time       `json:"indexedAt,omitempty"`

	// AI-derived fields
	DocType  string    `json:"docType,omitempty"`
	Tags     []string  `json:"tags,omitempty"`
	Summary  string    `json:"summary,omitempty"`
	KeyFacts []KeyFact `json:"keyFacts,omitempty"`

	// Scan specific fields
	ScanID        string `json:"scanID"`
	SequenceID    int    `json:"sequenceID"`
	ContentDigest string `json:"contentDigest,omitempty"`
}

// KeyFact is a single extracted label/value pair (e.g. amount due, due date, IBAN).
type KeyFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
