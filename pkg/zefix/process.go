package zefix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/go-datesfinder"

	"github.com/denysvitali/odi/pkg/models"
	"github.com/denysvitali/odi/zefix-tools/pkg/zefix"
)

var log = logrus.StandardLogger().WithField("package", "zefix")

type Processor struct {
	zefixClient *zefix.Client
}

func New(zefixDsn string) (*Processor, error) {
	if IsDisabledDSN(zefixDsn) {
		return &Processor{}, nil
	}
	zefixClient, err := zefix.New(zefixDsn)
	if err != nil {
		return nil, err
	}

	p := Processor{
		zefixClient: zefixClient,
	}
	return &p, nil
}

func IsDisabledDSN(zefixDsn string) bool {
	zefixDsn = strings.TrimSpace(zefixDsn)
	return zefixDsn == "" || strings.Contains(zefixDsn, "user=disabled database=disabled")
}

func (p *Processor) ProcessFromOpenSearch(ctx context.Context, osClient *opensearchapi.Client, index string) error {
	// Go through all the documents in the index and update them
	// by adding some new fields

	// 1. Get all the documents from the index
	size := 1000
	searchBody := map[string]interface{}{
		"sort": []map[string]string{{"date": "asc"}},
		"size": size,
	}
	jsonBody, err := json.Marshal(searchBody)
	if err != nil {
		return fmt.Errorf("marshal search body: %w", err)
	}

	searchResp, err := osClient.Search(ctx, &opensearchapi.SearchReq{
		Indices: []string{index},
		Body:    bytes.NewReader(jsonBody),
	})
	if err != nil {
		return fmt.Errorf("search request: %w", err)
	}

	// Parse response as JSON
	var result OpensearchResult[models.Document]
	dec := json.NewDecoder(searchResp.Inspect().Response.Body)
	err = dec.Decode(&result)
	if err != nil {
		return fmt.Errorf("decode search response: %w", err)
	}
	searchResp.Inspect().Response.Body.Close()

	// 2. Iterate over the documents
	for _, hit := range result.Hits.Hits {
		newSource := p.processText(hit.Source)
		newSourceBytes, err := json.Marshal(newSource)
		if err != nil {
			return err
		}

		// 3. Update the document
		indexResp, err := osClient.Index(ctx, opensearchapi.IndexReq{
			Index:      index,
			DocumentID: hit.ID,
			Body:       strings.NewReader(string(newSourceBytes)),
		})
		if err != nil {
			return fmt.Errorf("index request: %w", err)
		}
		indexResp.Inspect().Response.Body.Close()
	}

	return nil
}

func (p *Processor) processText(document models.Document) models.Document {
	// Find dates
	dates, errors := datesfinder.FindDates(document.Text)
	printErrors(errors)
	log.Infof("found %d dates", len(dates))

	// Find companies
	companies := p.FindCompanies(document.Text)
	log.Infof("found %d companies", len(companies))

	if len(companies) > 0 {
		document.Company = &companies[0]
		document.Companies = companies
	}

	if len(dates) > 0 {
		document.Date = &dates[0]
		document.Dates = dates
	}

	return document
}

var companyRegexp = regexp.MustCompile("(?i)([A-zü() -]+) (?:AG|GmbH|SA|Sagl)")

func (p *Processor) FindCompanies(text string) []zefix.Company {
	if p.zefixClient == nil {
		return nil
	}
	companiesMap := make(map[string]zefix.Company)
	res := companyRegexp.FindAllStringSubmatch(text, -1)
	for _, company := range res {
		companyName := strings.TrimSpace(company[0])
		if _, ok := companiesMap[companyName]; ok {
			continue
		}
		if strings.HasPrefix(strings.ToLower(companyName), strings.ToLower("Post CH ")) {
			// Skip Post CH AG since it does appear on basically every document
			continue
		}
		log.Infof("found company: %s", companyName)
		c, err := p.zefixClient.FindCompany(companyName)
		if err != nil {
			log.Warnf("error while fetching company: %s", err)
			continue
		}

		if c != nil {
			companiesMap[c.LegalName] = *c
			log.Infof("Adding company: %v", c.LegalName)
		}
	}

	var companies []zefix.Company
	for _, c := range companiesMap {
		companies = append(companies, c)
	}

	return companies
}

func (p *Processor) Ping() error {
	if p.zefixClient == nil {
		return nil
	}
	return p.zefixClient.Ping()
}

func printErrors(errors []error) {
	if len(errors) != 0 {
		log.Warnf("found %d errors", len(errors))
		for _, err := range errors {
			log.Warnf("error: %s", err)
		}
	}
}
