package middleware

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/jozefvalachovic/logger/v4"
)

// GRPCOptions configures gRPC request logging.
type GRPCOptions struct {
	LogPayloads bool
	SkipMethods []string
}

// GRPCOption is a functional option for gRPC logging.
type GRPCOption func(*GRPCOptions)

// WithGRPCLogPayloads enables logging of request/response payloads.
func WithGRPCLogPayloads(enabled bool) GRPCOption {
	return func(o *GRPCOptions) { o.LogPayloads = enabled }
}

// WithGRPCSkipMethods skips logging for the specified gRPC full methods.
func WithGRPCSkipMethods(methods ...string) GRPCOption {
	return func(o *GRPCOptions) { o.SkipMethods = methods }
}

// LogGRPCUnary logs a unary gRPC call. The handler function should invoke the
// actual gRPC handler. Returns the response and error from the handler.
//
// This function provides the logging logic without importing google.golang.org/grpc.
// Use it inside your own grpc.UnaryServerInterceptor:
//
//	func loggingUnary(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
//	    return middleware.LogGRPCUnary(ctx, info.FullMethod, func(ctx context.Context) (any, error) {
//	        return handler(ctx, req)
//	    })
//	}
//	server := grpc.NewServer(grpc.UnaryInterceptor(loggingUnary))
func LogGRPCUnary(ctx context.Context, fullMethod string, handler func(ctx context.Context) (any, error), opts ...GRPCOption) (any, error) {
	options := &GRPCOptions{}
	for _, o := range opts {
		o(options)
	}

	if slices.Contains(options.SkipMethods, fullMethod) {
		return handler(ctx)
	}

	start := time.Now()
	logger.LogDebug(fmt.Sprintf("gRPC %s started", fullMethod), "grpc.method", fullMethod)

	resp, err := handler(ctx)
	duration := time.Since(start)

	kv := []any{
		"grpc.method", fullMethod,
		"grpc.duration", duration.String(),
	}

	if err != nil {
		kv = append(kv, "grpc.error", err.Error())
		logger.LogError(fmt.Sprintf("gRPC %s failed %s", fullMethod, duration), kv...)
	} else {
		logger.LogInfo(fmt.Sprintf("gRPC %s completed %s", fullMethod, duration), kv...)
	}

	return resp, err
}

// LogGRPCStream logs a streaming gRPC call. The handler function should invoke the
// actual stream handler. Returns the error from the handler.
//
// Usage inside a grpc.StreamServerInterceptor:
//
//	func loggingStream(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
//	    return middleware.LogGRPCStream(ss.Context(), info.FullMethod, func(ctx context.Context) error {
//	        return handler(srv, ss)
//	    })
//	}
func LogGRPCStream(ctx context.Context, fullMethod string, handler func(ctx context.Context) error, opts ...GRPCOption) error {
	options := &GRPCOptions{}
	for _, o := range opts {
		o(options)
	}

	if slices.Contains(options.SkipMethods, fullMethod) {
		return handler(ctx)
	}

	start := time.Now()
	logger.LogDebug(fmt.Sprintf("gRPC stream %s started", fullMethod), "grpc.method", fullMethod)

	err := handler(ctx)
	duration := time.Since(start)

	kv := []any{
		"grpc.method", fullMethod,
		"grpc.duration", duration.String(),
	}

	if err != nil {
		kv = append(kv, "grpc.error", err.Error())
		logger.LogError(fmt.Sprintf("gRPC stream %s failed %s", fullMethod, duration), kv...)
	} else {
		logger.LogInfo(fmt.Sprintf("gRPC stream %s completed %s", fullMethod, duration), kv...)
	}

	return err
}
