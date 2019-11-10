package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/mchmarny/gcputil/metric"
)

const (
	// custom metrics dimensions
	invocationMetric = "invocation"
	messagesMetric   = "message"
	durationMetric   = "duration"
)

func pump(sub, ds, table string) (count int, err error) {

	if sub == "" || ds == "" || table == "" {
		return 0, fmt.Errorf(
			"missing required parameter: (sub=%s, ds=%s, table=%s)",
			sub, ds, table)
	}

	ctx := context.Background()
	start := time.Now()

	logger.Printf("creating pubsub client[%s]", projectID)
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return 0, fmt.Errorf("pubsub client[%s]: %v",
			projectID, err)
	}

	logger.Printf("creating importer[%s.%s.%s]",
		projectID, ds, table)
	imp, err := newImportClient(ctx, ds, table)
	if err != nil {
		return 0, fmt.Errorf("bigquery client[%s.%s]: %v",
			ds, table, err)
	}

	logger.Printf("creating pubsub subscription[%s]", sub)
	s := client.Subscription(sub)
	inCtx, cancel := context.WithCancel(ctx)
	var mu sync.Mutex
	messageCounter := 0
	totalCounter := 0
	var innerError error
	lastMessage := time.Now()

	// this will cancel the sub receive loop if max stall time has reached
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				elapsed := int(time.Now().Sub(lastMessage).Seconds())
				if elapsed > maxStall {
					logger.Println("max stall time reached")
					cancel()
					ticker.Stop()
					return
				}
			}
		}
	}()

	// start pulling messages from subscription
	receiveErr := s.Receive(inCtx, func(ctx context.Context, msg *pubsub.Message) {

		lastMessage = time.Now()

		mu.Lock()
		defer mu.Unlock()

		messageCounter++
		totalCounter++

		// append message to the importer
		appendErr := imp.append(msg.Data)
		if appendErr != nil {
			logger.Printf("error on data append: %v", appendErr)
			innerError = appendErr
			return
		}

		msg.Ack() //TODO: Ack after inserts?

		// check whether time to exec the batch
		if messageCounter == batchSize {
			logger.Println("batch size reached")
			messageCounter = 0
			if insertErr := imp.insert(ctx); insertErr != nil {
				innerError = insertErr
				return
			}
		}

		// check if max job time has been reached
		elapsed := int(time.Now().Sub(start).Seconds())
		if elapsed > maxDuration {
			logger.Println("max job exec time reached")
			cancel()
		}

	}) // end revive

	// ticker times no longer needed
	ticker.Stop()

	// receive error
	if receiveErr != nil {
		return 0, fmt.Errorf("pubsub subscription[%s] receive: %v",
			sub, receiveErr)
	}

	// error inside of receive handler
	if innerError != nil {
		return 0, fmt.Errorf("pubsub receive[%s] process error: %v",
			sub, innerError)
	}

	// insert leftovers
	if insertErr := imp.insert(ctx); insertErr != nil {
		return 0, fmt.Errorf("bigquery insert[%s] error: %v",
			sub, insertErr)
	}

	// metrics
	totalDuration := time.Now().Sub(start).Seconds()
	if metricErr := submitMetrics(ctx, sub, totalCounter, totalDuration); metricErr != nil {
		return 0, fmt.Errorf("metrics[%s] error: %v",
			sub, metricErr)
	}

	return totalCounter, nil
}

func submitMetrics(ctx context.Context, id string, c int, d float64) error {
	m, err := metric.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("metric client[%s]: %v", projectID, err)
	}

	if err = m.Publish(ctx, id, invocationMetric, 1); err != nil {
		return fmt.Errorf("metric record[%s][%s]: %v", id, invocationMetric, err)
	}

	if err = m.Publish(ctx, id, messagesMetric, c); err != nil {
		return fmt.Errorf("metric record[%s][%s]: %v", id, messagesMetric, err)
	}

	if err = m.Publish(ctx, id, durationMetric, d); err != nil {
		return fmt.Errorf("metric record[%s][%s]: %v", id, durationMetric, err)
	}

	return nil
}
