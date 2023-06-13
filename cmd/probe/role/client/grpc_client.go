package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"
	"unicode"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	flAddr        string
	flService     string
	flConnTimeout time.Duration
	flRPCHeaders  = rpcHeaders{MD: make(metadata.MD)}
	flRPCTimeout  time.Duration
)

const (
	// StatusInvalidArguments indicates specified invalid arguments.
	StatusInvalidArguments = 1
	// StatusConnectionFailure indicates connection failed.
	StatusConnectionFailure = 2
	// StatusRPCFailure indicates rpc failed.
	StatusRPCFailure = 3
	// StatusUnhealthy indicates rpc succeeded but indicates unhealthy service.
	StatusUnhealthy = 4
)

func init() {
	flagSet := flag.NewFlagSet("", flag.ContinueOnError)
	log.SetFlags(0)
	flagSet.StringVar(&flAddr, "addr", "", "(required) tcp host:port to connect")
	flagSet.StringVar(&flService, "service", "", "service name to check (default: \"\")")

	// timeouts
	flagSet.DurationVar(&flConnTimeout, "connect-timeout", time.Second, "timeout for establishing connection")
	flagSet.Var(&flRPCHeaders, "rpc-header", "additional RPC headers in 'name: value' format. May specify more than one via multiple flags.")
	flagSet.DurationVar(&flRPCTimeout, "rpc-timeout", time.Second, "timeout for health check rpc")

	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		os.Exit(StatusInvalidArguments)
	}

	argError := func(s string, v ...interface{}) {
		log.Printf("error: "+s, v...)
		os.Exit(StatusInvalidArguments)
	}

	if flAddr == "" {
		argError("-addr not specified")
	}
	if flConnTimeout <= 0 {
		argError("-connect-timeout must be greater than zero (specified: %v)", flConnTimeout)
	}
	if flRPCTimeout <= 0 {
		argError("-rpc-timeout must be greater than zero (specified: %v)", flRPCTimeout)
	}
}

type rpcHeaders struct{ metadata.MD }

func (s *rpcHeaders) String() string { return fmt.Sprintf("%v", s.MD) }

func (s *rpcHeaders) Set(value string) error {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid RPC header, expected 'key: value', got %q", value)
	}
	trimmed := strings.TrimLeftFunc(parts[1], unicode.IsSpace)
	s.Append(parts[0], trimmed)
	return nil
}

func main() {
	retcode := 0
	defer func() { os.Exit(retcode) }()

	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		sig := <-c
		if sig == os.Interrupt {
			log.Printf("cancellation received")
			cancel()
			return
		}
	}()

	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	opts = append(opts, grpc.WithInsecure())

	dialCtx, dialCancel := context.WithTimeout(ctx, flConnTimeout)
	defer dialCancel()
	conn, err := grpc.DialContext(dialCtx, flAddr, opts...)
	if err != nil {
		if err == context.DeadlineExceeded {
			log.Printf("timeout: failed to connect service %q within %v", flAddr, flConnTimeout)
		} else {
			log.Printf("error: failed to connect service at %q: %+v", flAddr, err)
		}
		retcode = StatusConnectionFailure
		return
	}

	defer conn.Close()

	rpcCtx, rpcCancel := context.WithTimeout(ctx, flRPCTimeout)
	defer rpcCancel()
	rpcCtx = metadata.NewOutgoingContext(rpcCtx, flRPCHeaders.MD)
	resp, err := healthpb.NewHealthClient(conn).Check(rpcCtx,
		&healthpb.HealthCheckRequest{
			Service: flService})
	if err != nil {
		if stat, ok := status.FromError(err); ok && stat.Code() == codes.DeadlineExceeded {
			log.Printf("timeout: health rpc did not complete within %v", flRPCTimeout)
		} else {
			log.Printf("error: health rpc failed: %+v", err)
		}
		retcode = StatusRPCFailure
		return
	}

	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		log.Printf("service unhealthy (responded with %q)", resp.GetStatus().String())
		retcode = StatusUnhealthy
		return
	}

	log.Printf("status: %v", resp.GetStatus().String())
}
