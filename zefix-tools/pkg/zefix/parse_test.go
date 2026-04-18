package zefix_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/denysvitali/sparql-client"
)

func TestParse(t *testing.T) {
	const fixture = "../../zefix.json"
	f, err := os.Open(fixture)
	if os.IsNotExist(err) {
		t.Skipf("fixture %s not present; skipping", fixture)
	}
	if err != nil {
		t.Fatalf("unable to open file: %v", err)
	}

	var result sparql.Result
	dec := json.NewDecoder(f)
	err = dec.Decode(&result)
	if err != nil {
		t.Fatalf("unable to decode file: %v", err)
	}

	for _, v := range result.Results.Bindings {
		if v["name"].Value == "KPT Assicurazioni SA" {
			t.Logf("Found: %v", v)
		}
	}
}
