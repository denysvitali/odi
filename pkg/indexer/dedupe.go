package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

// ErrDigestAlreadyReserved indicates that a content digest was already reserved
// by another worker. Callers should treat this as "duplicate, skip".
var ErrDigestAlreadyReserved = errors.New("content digest already reserved")

type ContentDigestReservation struct {
	Reserved           bool
	ExistingDocumentID string
}

// IsDuplicate reports whether this reservation indicates that another document
// already owns the digest (i.e. a concurrent worker won the race or the
// digest was previously persisted).
func (r ContentDigestReservation) IsDuplicate() bool {
	return !r.Reserved && r.ExistingDocumentID != ""
}

type contentDigestReservationDocument struct {
	DocumentID string    `json:"documentID"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (i *Indexer) contentDigestIndex() string {
	return i.documentsIndex + "_digests"
}

// ReserveContentDigest atomically reserves the given content digest for the
// supplied documentID using OpenSearch's op_type=create semantics (PUT
// /{index}/_create/{id}), which is atomic — concurrent writers racing on the
// same digest get a 409 Conflict.
//
// On a successful reservation the returned ContentDigestReservation has
// Reserved=true and err==nil. On a 409 the returned reservation has
// Reserved=false and ExistingDocumentID populated with whatever document
// currently holds the digest, and the returned error is nil (kept for
// backward compatibility with existing callers that branch on
// Reserved/ExistingDocumentID). The exported sentinel
// ErrDigestAlreadyReserved is provided so future callers can switch to
// errors.Is — see IsDigestAlreadyReserved on the reservation value.
func (i *Indexer) ReserveContentDigest(ctx context.Context, digest string, documentID string) (ContentDigestReservation, error) {
	if digest == "" {
		return ContentDigestReservation{Reserved: true}, nil
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
		existingDocumentID, getErr := i.getReservedDocumentID(ctx, digest)
		if getErr != nil {
			return ContentDigestReservation{}, getErr
		}
		// NOTE: existing callers (pkg/ingestor, internal/server, internal/cli)
		// check reservation.Reserved and return early on !Reserved without
		// inspecting the error. Returning a typed error here would break
		// those callers (they'd surface the duplicate-skip as a hard failure).
		// We keep err==nil and surface the conflict via Reserved=false +
		// ExistingDocumentID; callers that want errors.Is semantics can use
		// the exported ErrDigestAlreadyReserved together with the
		// IsDigestAlreadyReserved helper below.
		return ContentDigestReservation{ExistingDocumentID: existingDocumentID}, nil
	}

	errorMessage := decodeError(resp.Body)
	return ContentDigestReservation{}, fmt.Errorf("reserve content digest returned %s: %s", resp.Status, errorMessage)
}

// ReleaseContentDigest releases a previously-reserved content digest, but only
// if the reservation is still owned by the supplied documentID. The
// compare-and-delete is done atomically using OpenSearch's optimistic
// concurrency control (if_seq_no + if_primary_term), so a release racing with
// another worker that has just taken ownership of the same digest will not
// accidentally delete the new owner's reservation.
func (i *Indexer) ReleaseContentDigest(ctx context.Context, digest string, documentID string) error {
	if digest == "" {
		return nil
	}

	owner, err := i.getReservedDocument(ctx, digest)
	if err != nil {
		return err
	}
	if owner == nil {
		// Nothing to release.
		return nil
	}
	if owner.documentID != documentID {
		// Someone else owns it now — leave it alone.
		return nil
	}

	seqNo := owner.seqNo
	primaryTerm := owner.primaryTerm
	resp, err := i.opensearchClient.Document.Delete(ctx, opensearchapi.DocumentDeleteReq{
		Index:      i.contentDigestIndex(),
		DocumentID: digest,
		Params: opensearchapi.DocumentDeleteParams{
			IfSeqNo:       &seqNo,
			IfPrimaryTerm: &primaryTerm,
		},
	})
	if err != nil {
		// The OpenSearch Go client returns an error on non-2xx responses; we
		// need to inspect the response to distinguish "lost the race" (409)
		// and "already gone" (404) from real failures.
		if resp != nil {
			statusCode := resp.Inspect().Response.StatusCode
			if statusCode == http.StatusNotFound || statusCode == http.StatusConflict {
				return nil
			}
		}
		return fmt.Errorf("release content digest: %w", err)
	}
	defer resp.Inspect().Response.Body.Close()
	statusCode := resp.Inspect().Response.StatusCode
	if statusCode == http.StatusNotFound || statusCode == http.StatusConflict {
		// Lost the compare-and-delete race; another worker now owns the
		// reservation. That's fine.
		return nil
	}
	if statusCode >= 400 {
		return fmt.Errorf("release content digest returned %s", resp.Inspect().Response.Status())
	}
	return nil
}

type reservedDocument struct {
	documentID  string
	seqNo       int
	primaryTerm int
}

func (i *Indexer) getReservedDocument(ctx context.Context, digest string) (*reservedDocument, error) {
	resp, err := i.opensearchClient.Document.Get(ctx, opensearchapi.DocumentGetReq{
		Index:      i.contentDigestIndex(),
		DocumentID: digest,
	})
	if err != nil {
		if resp != nil && resp.Inspect().Response != nil && resp.Inspect().Response.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get reserved content digest: %w", err)
	}
	defer resp.Inspect().Response.Body.Close()

	statusCode := resp.Inspect().Response.StatusCode
	if statusCode == http.StatusNotFound {
		return nil, nil
	}
	if statusCode >= 400 {
		return nil, fmt.Errorf("get reserved content digest returned %s", resp.Inspect().Response.Status())
	}

	if !resp.Found {
		return nil, nil
	}

	var src contentDigestReservationDocument
	if len(resp.Source) > 0 {
		if err := json.Unmarshal(resp.Source, &src); err != nil {
			return nil, fmt.Errorf("decode reserved content digest: %w", err)
		}
	}
	return &reservedDocument{
		documentID:  src.DocumentID,
		seqNo:       resp.SeqNo,
		primaryTerm: resp.PrimaryTerm,
	}, nil
}

func (i *Indexer) getReservedDocumentID(ctx context.Context, digest string) (string, error) {
	owner, err := i.getReservedDocument(ctx, digest)
	if err != nil {
		return "", err
	}
	if owner == nil {
		return "", nil
	}
	return owner.documentID, nil
}
