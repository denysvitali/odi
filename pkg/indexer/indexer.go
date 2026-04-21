package indexer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/go-datesfinder"
	swissqrcode "github.com/denysvitali/go-swiss-qr-bill"

	"github.com/denysvitali/odi/pkg/models"
	"github.com/denysvitali/odi/pkg/ocrclient"
	"github.com/denysvitali/odi/pkg/ocrclient/caroundtripper"
	"github.com/denysvitali/odi/pkg/ocrtext"
	"github.com/denysvitali/odi/pkg/zefix"
)

type Indexer struct {
	opensearchAddr               string
	opensearchUsername           string
	opensearchPassword           string
	opensearchInsecureSkipVerify bool
	documentsIndex               string
	ocrAPIAddr                   string
	ocrAPICaPath                 string
	zefixDsn                     string

	opensearchClient *opensearchapi.Client
	ocrClient        *ocrclient.Client
	zefixProcessor   *zefix.Processor

	initCalled         bool
	mergeDistance      float64
	horizontalDistance float64
}

const (
	DefaultDocumentsIndex = "documents"

	// DefaultMergeDistance is the default maximum vertical distance between text blocks to be merged
	DefaultMergeDistance = 150
	// DefaultHorizontalDistance is the default maximum horizontal distance between text blocks to be merged
	DefaultHorizontalDistance = 10
)

type Option func(*Indexer)

var log = logrus.StandardLogger().WithField("package", "indexer")

func New(opensearchAddr string, ocrAPIAddr string, zefixDsn string, opts ...Option) (*Indexer, error) {
	idx := &Indexer{
		opensearchAddr:     opensearchAddr,
		ocrAPIAddr:         ocrAPIAddr,
		zefixDsn:           zefixDsn,
		documentsIndex:     DefaultDocumentsIndex,
		mergeDistance:      DefaultMergeDistance,
		horizontalDistance: DefaultHorizontalDistance,
	}
	for _, opt := range opts {
		opt(idx)
	}
	if err := idx.init(); err != nil {
		return nil, fmt.Errorf("init indexer: %w", err)
	}
	return idx, nil
}

func (i *Indexer) PingOcrApi() (bool, error) {
	err := i.ensureOcrApiClient()
	if err != nil {
		return false, fmt.Errorf("ensure OCR API client: %w", err)
	}

	return i.ocrClient.Healthz()
}

func (i *Indexer) PingOpensearch(ctx context.Context) (*opensearch.Response, error) {
	err := i.ensureOpensearchClient()
	if err != nil {
		return nil, fmt.Errorf("ensure opensearch client: %w", err)
	}

	return i.opensearchClient.Ping(ctx, &opensearchapi.PingReq{})
}

func (i *Indexer) ensureOpensearchClient() error {
	if i.opensearchClient != nil {
		return nil
	}

	var err error
	i.opensearchClient, err = opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: i.opensearchInsecureSkipVerify},
			},
			Addresses: []string{i.opensearchAddr},
			Username:  i.opensearchUsername,
			Password:  i.opensearchPassword,
		},
	})
	if err != nil {
		return fmt.Errorf("create opensearch client: %w", err)
	}
	return nil
}

func (i *Indexer) ensureOcrApiClient() error {
	if i.ocrClient != nil {
		return nil
	}

	var err error
	i.ocrClient, err = ocrclient.New(i.ocrAPIAddr)
	if err != nil {
		return fmt.Errorf("create OCR client: %w", err)
	}

	if i.ocrAPICaPath != "" {
		caRoundTripper, err := caroundtripper.New(i.ocrAPICaPath)
		if err != nil {
			return fmt.Errorf("create CA round tripper: %w", err)
		}
		i.ocrClient.SetHTTPTransport(caRoundTripper)
	}
	return nil
}

func (i *Indexer) init() error {
	err := i.ensureOcrApiClient()
	if err != nil {
		return fmt.Errorf("ocr client: %w", err)
	}
	err = i.ensureOpensearchClient()
	if err != nil {
		return fmt.Errorf("opensearchClient: %w", err)
	}

	err = i.ensureZefixClient()
	if err != nil {
		return fmt.Errorf("zefix client: %w", err)
	}

	// Create OpenSearch index
	err = i.createOpensearchIndex(context.Background())
	if err != nil {
		return fmt.Errorf("unable to create opensearch index: %w", err)
	}
	err = i.createContentDigestIndex(context.Background())
	if err != nil {
		return fmt.Errorf("unable to create content digest index: %w", err)
	}

	// Check if API ping works
	h, err := i.ocrClient.Healthz()
	if err != nil {
		return fmt.Errorf("unable to ping OCR API: %w", err)
	}

	if !h {
		return fmt.Errorf("OCR API is not healthy")
	}

	i.initCalled = true
	return nil
}

func (i *Indexer) ensureInitCalled() error {
	if !i.initCalled {
		return fmt.Errorf("init wasn't called")
	}
	return nil
}

func (i *Indexer) Index(ctx context.Context, page models.ScannedPage) error {
	log.Debugf("indexing %s", page.ID())
	err := i.ensureInitCalled()
	if err != nil {
		return fmt.Errorf("ensure init called: %w", err)
	}

	log.Debugf("processing %s via OCR client", page.ID())
	ocrResult, err := i.ocrClient.Process(ctx, page.Reader)
	if err != nil {
		return fmt.Errorf("ocr client failed: %w", err)
	}

	log.Debugf("getting text")
	var documentText string
	if ocrResult == nil || len(ocrResult.TextBlocks) == 0 {
		documentText = ""
	} else {
		documentText = i.getText(ocrResult)
	}
	log.Debugf("zefixProcessor finds the companies")
	zefixCompanies := i.zefixProcessor.FindCompanies(documentText)
	log.Debugf("found %d companies", len(zefixCompanies))

	jsonBuffer := bytes.NewBuffer(nil)
	enc := json.NewEncoder(jsonBuffer)
	log.Debugf("getting barcodes for %s", page.ID())
	barcodes := i.getBarcodes(ocrResult)
	var barcode *models.Barcode
	var additionalBarcodes []models.Barcode
	if len(barcodes) > 1 {
		additionalBarcodes = barcodes[1:]
	}

	if len(barcodes) >= 1 {
		barcode = &barcodes[0]
	}
	dates := getDocumentDates(ocrResult)
	d := &models.Document{
		Text:               documentText,
		Barcode:            barcode,
		AdditionalBarcodes: additionalBarcodes,
		IndexedAt:          time.Now(),
		ScanID:             page.ScanID,
		SequenceID:         page.SequenceID,
		ContentDigest:      page.ContentDigest,
	}
	if len(dates) > 0 {
		d.Date = &dates[0]
		d.Dates = dates
	}
	if len(zefixCompanies) > 0 {
		log.Debugf("found %d companies", len(zefixCompanies))
		d.Company = &zefixCompanies[0]
		d.Companies = zefixCompanies
	}
	err = enc.Encode(d)
	if err != nil {
		return fmt.Errorf("unable to encode JSON: %w", err)
	}

	log.Debugf("indexing %s", page.ID())

	indexResp, err := i.opensearchClient.Index(ctx, opensearchapi.IndexReq{
		Index:      i.documentsIndex,
		DocumentID: page.ID(),
		Body:       jsonBuffer,
	})
	if err != nil {
		return fmt.Errorf("index document: %w", err)
	}

	if indexResp.Inspect().Response.StatusCode < 200 || indexResp.Inspect().Response.StatusCode > 299 {
		errorMessage := decodeError(indexResp.Inspect().Response.Body)
		return fmt.Errorf("opensearch returned an invalid status %s: %s", indexResp.Inspect().Response.Status(), errorMessage)
	}
	log.Debugf("indexed %s", page.ID())
	return nil
}

func decodeError(body io.ReadCloser) string {
	var errorMessage struct {
		Error json.RawMessage `json:"error"`
	}
	dec := json.NewDecoder(body)
	if err := dec.Decode(&errorMessage); err != nil {
		return fmt.Sprintf("failed to decode error: %v", err)
	}
	if len(errorMessage.Error) == 0 {
		return ""
	}

	var plain string
	if err := json.Unmarshal(errorMessage.Error, &plain); err == nil {
		return plain
	}
	return string(errorMessage.Error)
}

// Given the result of the OCR, return the most likely date of the document
func getDocumentDates(result *ocrclient.OcrResult) []time.Time {
	// Try to parse the date from the text
	var dates []time.Time
	for _, t := range result.TextBlocks {
		d, err := datesfinder.FindDates(t.Text)
		if err != nil {
			continue
		}
		dates = append(dates, d...)
	}

	if len(dates) == 0 {
		return nil
	}
	return dates
}

func (i *Indexer) getBarcodes(result *ocrclient.OcrResult) []models.Barcode {
	if result == nil {
		return nil
	}

	var barcodes []models.Barcode
	for _, b := range result.Barcodes {
		if strings.HasPrefix(b.RawValue, "SPC") {
			// Try to parse Swiss QR Bill
			qrCode, err := swissqrcode.Decode(b.RawValue)
			if err != nil {
				log.Warnf("unable to decode Swiss QR Bill: %v", err)
				barcodes = append(barcodes, models.Barcode{Text: b.RawValue})
				continue
			}
			barcodes = append(barcodes, models.Barcode{QRBill: qrCode})
		} else {
			barcodes = append(barcodes, models.Barcode{Text: b.RawValue})
		}
	}
	return barcodes
}

func (i *Indexer) getText(result *ocrclient.OcrResult) string {
	return ocrtext.GetText(result, i.mergeDistance, i.horizontalDistance)
}

func (i *Indexer) createOpensearchIndex(ctx context.Context) error {
	exists, err := i.opensearchTargetExists(ctx, i.documentsIndex)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	resp, err := i.opensearchClient.Indices.Create(ctx, opensearchapi.IndicesCreateReq{Index: i.documentsIndex})
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer resp.Inspect().Response.Body.Close()

	statusCode := resp.Inspect().Response.StatusCode
	if statusCode == http.StatusOK || statusCode == http.StatusCreated {
		return nil
	}
	if statusCode == http.StatusBadRequest {
		errorMessage := decodeError(resp.Inspect().Response.Body)
		if strings.Contains(errorMessage, "resource_already_exists_exception") ||
			strings.Contains(errorMessage, "already exists as alias") {
			return nil
		}
		return fmt.Errorf("create index returned %s: %s", resp.Inspect().Response.Status(), errorMessage)
	}

	return fmt.Errorf("unexpected status %s", resp.Inspect().Response.Status())
}

func (i *Indexer) createContentDigestIndex(ctx context.Context) error {
	exists, err := i.opensearchTargetExists(ctx, i.contentDigestIndex())
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	body := strings.NewReader(`{
		"mappings": {
			"properties": {
				"documentID": { "type": "keyword" },
				"createdAt": { "type": "date" }
			}
		}
	}`)
	resp, err := i.opensearchClient.Indices.Create(ctx, opensearchapi.IndicesCreateReq{
		Index: i.contentDigestIndex(),
		Body:  body,
	})
	if err != nil {
		return fmt.Errorf("create content digest index: %w", err)
	}
	defer resp.Inspect().Response.Body.Close()

	statusCode := resp.Inspect().Response.StatusCode
	if statusCode == http.StatusOK || statusCode == http.StatusCreated {
		return nil
	}
	if statusCode == http.StatusBadRequest {
		errorMessage := decodeError(resp.Inspect().Response.Body)
		if strings.Contains(errorMessage, "resource_already_exists_exception") ||
			strings.Contains(errorMessage, "already exists as alias") {
			return nil
		}
		return fmt.Errorf("create content digest index returned %s: %s", resp.Inspect().Response.Status(), errorMessage)
	}

	return fmt.Errorf("unexpected status %s", resp.Inspect().Response.Status())
}

func (i *Indexer) opensearchTargetExists(ctx context.Context, name string) (bool, error) {
	resp, err := i.opensearchClient.Indices.Exists(ctx, opensearchapi.IndicesExistsReq{Indices: []string{name}})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, fmt.Errorf("check index or alias %s: %w", name, err)
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	if statusCode == http.StatusNotFound {
		return false, nil
	}
	if statusCode >= 400 {
		return false, fmt.Errorf("check index or alias %s returned %s", name, resp.Status())
	}

	return true, nil
}

func (i *Indexer) ensureZefixClient() error {
	var err error
	i.zefixProcessor, err = zefix.New(i.zefixDsn)
	if err != nil {
		return fmt.Errorf("create zefix client: %w", err)
	}
	return nil
}

func (i *Indexer) PingZefix() error {
	if zefix.IsDisabledDSN(i.zefixDsn) {
		return nil
	}
	return i.zefixProcessor.Ping()
}

func (i *Indexer) IsZefixConfigured() bool {
	return !zefix.IsDisabledDSN(i.zefixDsn)
}
