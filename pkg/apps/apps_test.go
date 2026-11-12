package apps

import (
	"testing"
)

func TestIndexerSearch(t *testing.T) {
	idx := NewIndexer()
	idx.Scan()

	// Search for terminal
	results := idx.Search("term")
	if len(results) == 0 {
		t.Fatal("Expected results for 'term', got none")
	}

	found := false
	for _, r := range results {
		if r.ID == "sys-terminal" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected sys-terminal in search results, got %+v", results)
	}

	// Search for notepad / bloc
	notepadResults := idx.Search("notep")
	if len(notepadResults) == 0 {
		t.Fatal("Expected results for 'notep', got none")
	}
	t.Logf("Found %d apps matching 'notep': %s", len(notepadResults), notepadResults[0].Name)
}
