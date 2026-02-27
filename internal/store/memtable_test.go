package store_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/fixed-partitioning/internal/store"
)

type Tuple struct {
	Keys   [][]byte
	Values [][]byte
}

func TestSkipList(t *testing.T) {
	testData := Tuple{
		Keys: [][]byte{
			[]byte("foo"),
			[]byte("bar"),
			[]byte("mock"),
		},
		Values: [][]byte{
			[]byte(`{
  				"_id": "65f1a2bc9d1e4f0012345678",
  				"name": "Alice Rossi",
  				"email": "alice.rossi@example.com",
  				"age": 29,
  				"active": true,
  				"roles": ["user", "editor"],
  				"profile": {
    				"bio": "Software engineer",
    				"location": "Pesaro, IT"
  				},
 				 "createdAt": "2026-02-27T10:00:00Z"
			}`),
			[]byte(`{
  				"_id": "65f1a2bc9d1e4f0012345679",
  				"name": "Bob Bianchi",
  				"email": "bob.bianchi@example.com",
  				"age": 42,
  				"active": false,
  				"roles": ["user"],
  				"profile": {
    				"bio": "Database administrator",
    				"location": "Milano, IT"
  				},
  				"createdAt": "2026-02-26T08:12:44Z"
			}`),
			[]byte(`{
  				"_id": "65f1a2bc9d1e4f0012345680",
  				"title": "SkipList Design",
  				"tags": ["database", "golang", "lsm"],
 				 "views": 1823,
  				"rating": 4.7,
  				"author": {
    				"id": "u123",
    				"name": "Carlo Verdi"
  				},
  				"published": true
			}`),
		},
	}

	table := store.NewMemTable()
	for index, key := range testData.Keys {
		value := testData.Values[index]
		table.AddDocument(key, value)

		err, result := table.FetchDocument(key)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(result, value) {
			t.Fatal(errors.New("something went wrong during insertion operation"))
		}

		err = table.DeleteDocument(key)
		if err != nil {
			t.Fatal(err)
		}
	}
}
