package meta

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDateAddedJSON(t *testing.T) {
	stamp := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	for _, added := range []*time.Time{nil, &stamp} {
		data, err := json.Marshal(Record{DateAdded: added})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"date_added"`) != (added != nil) {
			t.Fatalf("incorrect optional date_added field: %s", data)
		}
		var decoded Record
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if added != nil && (decoded.DateAdded == nil || !decoded.DateAdded.Equal(*added)) {
			t.Fatalf("date_added did not round trip: %v", decoded.DateAdded)
		}
	}
}
