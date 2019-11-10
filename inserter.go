package main

import (
	"context"
	"encoding/json"
	"sync"

	"cloud.google.com/go/bigquery"
	"github.com/google/uuid"
)

func newImportClient(ctx context.Context, ds, table string) (c *importClient, err error) {
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	inserter := client.Dataset(ds).Table(table).Inserter()
	inserter.IgnoreUnknownValues = true

	return &importClient{
		inserter: inserter,
		records:  make([]*simpleRecord, 0),
	}, nil
}

type simpleRecord map[string]bigquery.Value

func (rec simpleRecord) Save() (map[string]bigquery.Value, string, error) {
	return rec, uuid.New().String(), nil
}

type importClient struct {
	inserter *bigquery.Inserter
	records  []*simpleRecord
}

func (c *importClient) append(data []byte) error {
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()
	var rec simpleRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		logger.Printf("error unmarshaling %q\n", data)
		return err
	}
	c.records = append(c.records, &rec)
	return nil
}

func (c *importClient) clear() {
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()
	c.records = make([]*simpleRecord, 0)
}

func (c *importClient) insert(ctx context.Context) error {
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
