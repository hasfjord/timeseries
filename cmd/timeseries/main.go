package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/hasfjord/timeseries/internal/api"
	"github.com/hasfjord/timeseries/internal/influx"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Config struct {
	db          influx.Config
	GRPCAddress string `envconfig:"GRPC_ADDRESS"`
	HTTPAddress string `envconfig:"HTTP_ADDRESS"`
}

func main() {
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx)
	done()
	if err != nil {
		logrus.Fatal(err)
	}
}

func run(ctx context.Context) error {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		_ = envconfig.Usage("", &cfg)
		return err
	}

	eg, _ := errgroup.WithContext(ctx)

	lis, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		return err
	}

	var opts []grpc.ServerOption

	grpcServer := grpc.NewServer(opts...)

	dbClient := influx.NewClient(cfg.db)

	gRPCserver := api.NewTimeSeriesServiceServer(dbClient)

	gRPCserver.Register(grpcServer)

	eg.Go(func() error {
		logrus.Infof("gRPC server started, listening to %s", cfg.GRPCAddress)
		return grpcServer.Serve(lis)
	})

	http.HandleFunc("/readiness", HealthHandler)
	http.HandleFunc("/liveness", HealthHandler)
	httpServer := &http.Server{Addr: cfg.HTTPAddress,
		ReadTimeout:       time.Second * 15,
		ReadHeaderTimeout: time.Second * 10}

	eg.Go(func() error {
		logrus.Infof("http server started, listening to %s", cfg.HTTPAddress)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	// Wait for signal, or error:
	<-ctx.Done()
	logrus.Info("worker: shutting down gRPC server...")
	grpcServer.GracefulStop()

	// Shut down the http server with a 5s timeout
	logrus.Info("worker: shutting down http server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Wait for all goroutines to shut down
	logrus.Info("waiting for goroutines to finish...")
	return eg.Wait()
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
