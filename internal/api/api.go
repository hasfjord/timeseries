package api

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hasfjord/timeseries/internal/api/gen/v1"
	"github.com/hasfjord/timeseries/internal/influx"
	"google.golang.org/grpc"
)

type TimeSeriesServiceServer struct {
	gen.UnimplementedTimeSeriesServiceServer
	dbClient
}

func NewTimeSeriesServiceServer(databaseClient dbClient) TimeSeriesServiceServer {
	return TimeSeriesServiceServer{
		dbClient: databaseClient,
	}
}

type dbClient interface {
	Write(ctx context.Context, measurement influx.Measurement) error
}

func (s *TimeSeriesServiceServer) PostMeasurement(ctx context.Context, in *gen.PostMeasurementRequest) (*gen.PostMeasurementResponse, error) {
	ID := uuid.New().String()

	measurement := influx.Measurement{
		Name:      in.Measurement.Name,
		Type:      in.Type,
		Value:     in.Measurement.Value,
		Timestamp: time.Unix(in.Measurement.Timestamp, 0),
		Tags:      in.Measurement.Tags,
	}

	measurement.Tags["id"] = ID

	if err := s.Write(ctx, measurement); err != nil {
		return nil, err
	}

	return &gen.PostMeasurementResponse{Id: ID}, nil
}

func (s *TimeSeriesServiceServer) Register(srv *grpc.Server) {
	gen.RegisterTimeSeriesServiceServer(srv, s)
}
