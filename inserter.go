package main

import (
	"context"
	"encoding/json"
	"sync"

	"cloud.google.com/go/bigquery"
	"github.com/google/uuid"
)

func NewImportClient(ctx context.Context, ds, table string) (c *ImportClient, err error) {
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	inserter := client.Dataset(ds).Table(table).Inserter()
	inserter.IgnoreUnknownValues = true

	return &ImportClient{
		inserter: inserter,
		records:  make([]*simpleRecord, 0),
	}, nil
}

type simpleRecord map[string]bigquery.Value

func (rec simpleRecord) Save() (map[string]bigquery.Value, string, error) {
	return rec, uuid.New().String(), nil
}

type ImportClient struct {
	inserter *bigquery.Inserter
	records  []*simpleRecord
}

func (c *ImportClient) Append(data []byte) error {
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()
	var rec simpleRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		logger.Printf("error unmarshalling %q\n", data)
		return err
	}
	c.records = append(c.records, &rec)
	return nil
}

func (c *ImportClient) Clear() {
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()
	c.records = make([]*simpleRecord, 0)
}

func (c *ImportClient) Insert(ctx context.Context) error {
	if len(c.records) == 0 {
		logger.Println("nothing to insert")
		return nil
	}
	logger.Printf("inserting %d records...", len(c.records))
	if err := c.inserter.Put(ctx, c.records); err != nil {
		logger.Printf("error on put: %v", err)
		return err
	}
	return nil
}
