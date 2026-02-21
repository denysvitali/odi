package zefix

import (
	"encoding/json"
	"github.com/denysvitali/sparql-client"
	"gorm.io/gorm/clause"
	"os"
)

// Import takes a .json file returned from a SPARQL query to the Zefix API and
// imports it into the database
func (c *Client) Import(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	var result sparql.Result
	dec := json.NewDecoder(f)
	err = dec.Decode(&result)
	if err != nil {
		return err
	}

	var companies []Company

	for _, v := range result.Results.Bindings {
		comp := Company{}
		if b, ok := v["legal_name"]; ok {
			comp.LegalName = b.Value
		}
		if b, ok := v["name"]; ok {
			comp.Name = b.Value
		}
		if b, ok := v["company_uri"]; ok {
			comp.Uri = b.Value
		}
		if b, ok := v["locality"]; ok {
			comp.Locality = b.Value
		}
		if b, ok := v["type"]; ok {
			comp.Type = b.Value
		}
		if b, ok := v["addresse"]; ok {
			comp.Address = b.Value
		}
		companies = append(companies, comp)
	}

	tx := c.db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(companies, 1000)
	return tx.Error
}
