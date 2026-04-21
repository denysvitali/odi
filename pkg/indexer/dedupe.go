package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

type ContentDigestReservation struct {
	Reserved           bool
	ExistingDocumentID string
}

type contentDigestReservationDocument struct {
	DocumentID string    `json:"documentID"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (i *Indexer) contentDigestIndex() string {
	return i.documentsIndex + "_digests"
}

func (i *Indexer) ReserveContentDigest(ctx context.Context, digest string, documentID string) (ContentDigestReservation, error) {
	if digest == "" {
		return ContentDigestReservation{Reserved: true}, nil
	}
	if err := i.ensureInitCalled(); err != nil {
		return ContentDigestReservation{}, fmt.Errorf("ensure init called: %w", err)
	}

	body := bytes.NewBuffer(nil)
	if err := json.NewEncoder(body).Encode(contentDigestReservationDocument{
		DocumentID: documentID,
		CreatedAt:  time.Now(),
	}); err != nil {
		return ContentDigestReservation{}, fmt.Errorf("encode content digest reservation: %w", err)
	}

	req, err := opensearchapi.DocumentCreateReq{
		Index:      i.contentDigestIndex(),
		DocumentID: digest,
		Body:       body,
		Params: opensearchapi.DocumentCreateParams{
			Refresh: "true",
		},
	}.GetRequest()
	if err != nil {
		return ContentDigestReservation{}, fmt.Errorf("build content digest reservation request: %w", err)
	}

	resp, err := i.opensearchClient.Client.Perform(req.WithContext(ctx))
	if err != nil {
		return ContentDigestReservation{}, fmt.Errorf("reserve content digest: %w", err)
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	if statusCode >= 200 && statusCode <= 299 {
		return ContentDigestReservation{Reserved: true}, nil
	}
	if statusCode == http.StatusConflict {
		existingDocumentID, err := i.getReservedDocumentID(ctx, digest)
		if err != nil {
			return ContentDigestReservation{}, err
		}
		return ContentDigestReservation{ExistingDocumentID: existingDocumentID}, nil
	}

	errorMessage := decodeError(resp.Body)
	return ContentDigestReservation{}, fmt.Errorf("reserve content digest returned %s: %s", resp.Status, errorMessage)
}

func (i *Indexer) ReleaseContentDigest(ctx context.Context, digest string, documentID string) error {
	if digest == "" {
		return nil
	}

	existingDocumentID, err := i.getReservedDocumentID(ctx, digest)
	if err != nil {
		return err
	}
	if existingDocumentID != documentID {
		return nil
	}

	resp, err := i.opensearchClient.Document.Delete(ctx, opensearchapi.DocumentDeleteReq{
		Index:      i.contentDigestIndex(),
		DocumentID: digest,
	})
	if err != nil {
		return fmt.Errorf("release content digest: %w", err)
	}
	defer resp.Inspect().Response.Body.Close()
	if resp.Inspect().Response.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.Inspect().Response.StatusCode >= 400 {
		return fmt.Errorf("release content digest returned %s", resp.Inspect().Response.Status())
	}
	return nil
}

func (i *Indexer) getReservedDocumentID(ctx context.Context, digest string) (string, error) {
	resp, err := i.opensearchClient.Document.Get(ctx, opensearchapi.DocumentGetReq{
		Index:      i.contentDigestIndex(),
		DocumentID: digest,
	})
	if err != nil {
		return "", fmt.Errorf("get reserved content digest: %w", err)
	}
	defer resp.Inspect().Response.Body.Close()

	if resp.Inspect().Response.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.Inspect().Response.StatusCode >= 400 {
		return "", fmt.Errorf("get reserved content digest returned %s", resp.Inspect().Response.Status())
	}

	var doc struct {
		Source contentDigestReservationDocument `json:"_source"`
	}
	if err := json.NewDecoder(resp.Inspect().Response.Body).Decode(&doc); err != nil {
		return "", fmt.Errorf("decode reserved content digest: %w", err)
	}
	return doc.Source.DocumentID, nil
}
