package influx

import (
	"context"
	"fmt"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Token  string `envconfig:"INFLUX_TOKEN" required:"true"`
	URL    string `envconfig:"INFLUX_URL" required:"true"`
	Org    string `envconfig:"INFLUX_ORG" required:"true"`
	Bucket string `envconfig:"INFLUX_BUCKET" required:"true"`
}

type InfluxClient struct {
	i *influxdb2.Client
	w api.WriteAPIBlocking
}

func NewClient(cfg Config) InfluxClient {
	// influx client does not provide errors, so use recover to add observability
	defer func() {
		if r := recover(); r != nil {
			fields := logrus.Fields{"URL": cfg.URL, "Bucket": cfg.Bucket}
			logrus.WithFields(fields).Errorf("failed to create new influx client: %v", r)
		}
	}()
	client := influxdb2.NewClient(cfg.URL, cfg.Token)

	writeAPI := client.WriteAPIBlocking(cfg.Org, cfg.Bucket)

	return InfluxClient{
		i: &client,
		w: writeAPI,
	}
}

type Measurement struct {
	Name      string
	Type      string
	Value     float64
	Timestamp time.Time
	Tags      map[string]string
}

func (c InfluxClient) Write(ctx context.Context, measurement Measurement) error {

	if err := validateMeasurement(measurement); err != nil {
		return err
	}

	fields := map[string]interface{}{
		measurement.Type: measurement.Value,
	}

	point := write.NewPoint(measurement.Name, measurement.Tags, fields, measurement.Timestamp)
	if err := c.w.WritePoint(ctx, point); err != nil {
		return err
	}

	return nil
}

func validateMeasurement(measurement Measurement) error {
	if measurement.Name == "" {
		return fmt.Errorf("measurement name is required")
	}

	if measurement.Type == "" {
		return fmt.Errorf("measurement type is required")
	}

	if measurement.Timestamp.IsZero() {
		return fmt.Errorf("measurement timestamp cannot be zero")
	}

	return nil
}
