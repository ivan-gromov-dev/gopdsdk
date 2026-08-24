package jsoncodec

import (
	"strings"
	"testing"

	pdjson "github.com/ivan-gromov-dev/gopdsdk/playdate/json"
)

func TestAcceptanceSchemaAndBoundedEncoding(t *testing.T) {
	value, err := pdjson.Decode(strings.NewReader(`{"title":"Crank & Key","level":3,"flags":[true,false]}`), pdjson.Limits{MaxBytes: 128})
	if err != nil {
		t.Fatal(err)
	}
	g := New().(*game)
	if err := g.accept(value); err != nil {
		t.Fatal(err)
	}
	if g.summary != "JSON: Crank & Key L3 flags 2 bytes 70" {
		t.Fatalf("summary=%q", g.summary)
	}
}
