package metric

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/pkg/errors"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	googlepb "github.com/golang/protobuf/ptypes/timestamp"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"

	"github.com/mchmarny/gcputil/project"
)

const (
	metricTypePrefix = "custom.googleapis.com/metric"
)

var (
	logger = log.New(os.Stdout, "", 0)
)

// Client represents metric client
type Client struct {
	projectID    string
	sourceID     string
	metricClient *monitoring.MetricClient
}

// NewClientWithSource instantiates client with context and source ID
func NewClientWithSource(ctx context.Context, sourceID string) (client *Client, err error) {

	c, e := NewClient(ctx)
	if e != nil {
		return nil, errors.Wrap(e, "Error creating metric client with NewClientWithSource")
	}
	c.sourceID = sourceID
	return c, nil

}

// NewClient instantiates client
func NewClient(ctx context.Context) (client *Client, err error) {

	// get project ID
	p, err := project.GetID()
	if err != nil {
		return nil, errors.Wrap(err, "Error while getting project ID")
	}

	// create metric client
	mc, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating metric client with NewClient")
	}

	return &Client{
		projectID:    p,
		metricClient: mc,
	}, nil

}

// PublishForSource publishes time series based on the preconfigured metric and value to Stackdriver
// Example: `PublishForSource(ctx, "friction", 0.125)``
func (c *Client) PublishForSource(ctx context.Context, metricType string, metricValue interface{}) error {
	if c.sourceID == "" {
		return errors.New("Source ID not configured")
	}
	return c.Publish(ctx, c.sourceID, metricType, metricValue)
}

// CountForSource publishes time series based on metric +1 value to Stackdriver
func (c *Client) CountForSource(ctx context.Context, metricType string) error {
	if c.sourceID == "" {
		return errors.New("Source ID not configured")
	}
	return publish(ctx, c, c.sourceID, metricType, &monitoringpb.TypedValue{
		Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: float64(1)},
	})
}

// Publish publishes time series based on metric and value to Stackdriver
// Example: `Publish(ctx, "device1", "friction", 0.125)``
func (c *Client) Publish(ctx context.Context, sourceID, metricType string, metricValue interface{}) error {

	// derive typed value from passed interface
	// HACK: everything in stackdriver seems to be casting to double anyway so to avoid
	//       errors, capture the passed type and convert to double
	// https://github.com/census-instrumentation/opencensus-python/pull/696
	var val *monitoringpb.TypedValue
	switch v := metricValue.(type) {
	default:
		return errors.Errorf("Unsupported metric type: %T", v)
	case float32:
		val = &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: float64(metricValue.(float32))},
		}
	case float64:
		val = &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: metricValue.(float64)},
		}
	case int:
		val = &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: float64(metricValue.(int))},
		}
	case int32:
		val = &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: float64(metricValue.(int32))},
		}
	case int64:
		val = &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: float64(metricValue.(int64))},
		}
	}

	return publish(ctx, c, sourceID, metricType, val)

}

func publish(ctx context.Context, c *Client, sourceID, metricType string, metricValue *monitoringpb.TypedValue) error {

	// create data point
	ptTs := &googlepb.Timestamp{Seconds: time.Now().Unix()}
	dataPoint := &monitoringpb.Point{
		Interval: &monitoringpb.TimeInterval{StartTime: ptTs, EndTime: ptTs},
		Value:    metricValue,
	}

	// create time series request with the data point
	tsRequest := &monitoringpb.CreateTimeSeriesRequest{
		Name: monitoring.MetricProjectPath(c.projectID),
		TimeSeries: []*monitoringpb.TimeSeries{
			{
				Metric: &metricpb.Metric{
					Type: fmt.Sprintf("%s/%s", metricTypePrefix, metricType),
					Labels: map[string]string{
						"source_id": sourceID,
						// random label to work around SD complaining
						// about multiple events for same time window
						"random_label": fmt.Sprint(rand.Intn(100)),
					},
				},
				Resource: &monitoredrespb.MonitoredResource{
					Type:   "global",
					Labels: map[string]string{"project_id": c.projectID},
				},
				Points: []*monitoringpb.Point{dataPoint},
			},
		},
	}

	// publish series
	return c.metricClient.CreateTimeSeries(ctx, tsRequest)
}
