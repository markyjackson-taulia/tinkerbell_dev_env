package grpcserver

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"sync"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/packethost/pkg/log"
	"github.com/pkg/errors"
	"github.com/tinkerbell/tink/db"
	"github.com/tinkerbell/tink/metrics"
	"github.com/tinkerbell/tink/protos/events"
	"github.com/tinkerbell/tink/protos/hardware"
	"github.com/tinkerbell/tink/protos/template"
	"github.com/tinkerbell/tink/protos/workflow"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

var (
	logger         log.Logger
	grpcListenAddr = os.Getenv("TINKERBELL_GRPC_AUTHORITY")
)

// Server is the gRPC server for tinkerbell
type server struct {
	cert []byte
	modT time.Time

	db   db.Database
	quit <-chan struct{}

	dbLock  sync.RWMutex
	dbReady bool

	watchLock sync.RWMutex
	watch     map[string]chan string
}

// SetupGRPC setup and return a gRPC server
func SetupGRPC(ctx context.Context, log log.Logger, facility string, db *db.TinkDB, certBytes []byte, tlsCert tls.Certificate, modT time.Time, errCh chan<- error) {
	params := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.Creds(credentials.NewServerTLSFromCert(&tlsCert)),
	}
	logger = log
	metrics.SetupMetrics(facility, logger)
	server := &server{
		db:      db,
		dbReady: true,
		cert:    certBytes,
		modT:    modT,
	}

	// register servers
	s := grpc.NewServer(params...)
	template.RegisterTemplateServiceServer(s, server)
	workflow.RegisterWorkflowServiceServer(s, server)
	hardware.RegisterHardwareServiceServer(s, server)
	events.RegisterEventsServiceServer(s, server)
	reflection.Register(s)

	grpc_prometheus.Register(s)

	go func() {
		logger.Info("serving grpc")
		if grpcListenAddr == "" {
			grpcListenAddr = ":42113"
		}
		lis, err := net.Listen("tcp", grpcListenAddr)
		if err != nil {
			err = errors.Wrap(err, "failed to listen")
			logger.Error(err)
			panic(err)
		}

		errCh <- s.Serve(lis)
	}()

	go func() {
		<-ctx.Done()
		s.GracefulStop()
	}()
}
