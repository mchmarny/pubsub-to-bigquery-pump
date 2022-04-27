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

var (
	logger           = log.New(os.Stdout, "", 0)
	metricTypePrefix = "custom.googleapis.com/metric"
)

// Client represents metric client
type Client struct {
	projectID    string
	sourceID     string
	metricClient *monitoring.MetricClient
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

// MakeClient creates new metrics client or fails
func MakeClient(ctx context.Context) *Client {

	// get project ID
	p, err := project.GetID()
	if err != nil {
		logger.Fatalf("Error while getting project ID: %v", err)
	}

	// create metric client
	mc, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		logger.Fatalf("Error creating metric client with NewClient: %v", err)
	}

	return &Client{
		projectID:    p,
		metricClient: mc,
	}

}

// Publish publishes time series based on metric and value to Stackdriver
func (c *Client) Publish(ctx context.Context, metricType string, metricValue interface{}, labels map[string]string) error {

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

	return publish(ctx, c, metricType, val, labels)

}

func publish(ctx context.Context, c *Client, metricType string, metricValue *monitoringpb.TypedValue, labels map[string]string) error {

	// create data point
	ptTs := &googlepb.Timestamp{Seconds: time.Now().Unix()}
	dataPoint := &monitoringpb.Point{
		Interval: &monitoringpb.TimeInterval{StartTime: ptTs, EndTime: ptTs},
		Value:    metricValue,
	}

	// random label to work around SD complaining
	// about multiple events for same minute
	rand.Seed(time.Now().UnixNano())
	labels["random"] = fmt.Sprint(rand.Intn(1000))

	// create time series request with the data point
	tsRequest := &monitoringpb.CreateTimeSeriesRequest{
		Name: monitoring.MetricProjectPath(c.projectID),
		TimeSeries: []*monitoringpb.TimeSeries{
			{
				Metric: &metricpb.Metric{
					Type:   fmt.Sprintf("%s/%s", metricTypePrefix, metricType),
					Labels: labels,
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
