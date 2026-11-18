package grpcserver

import (
	"context"
	"database/sql"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	routerAPIKeyHeader            = "x-router-api-key"
	resolvedNodeKey    contextKey = "resolved_node_addr"
)

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

func withResolvedNodeAddr(ctx context.Context, addr string) context.Context {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ctx
	}
	return context.WithValue(ctx, resolvedNodeKey, addr)
}

func resolvedNodeAddrFromContext(ctx context.Context) string {
	addr, _ := ctx.Value(resolvedNodeKey).(string)
	return strings.TrimSpace(addr)
}

func (s *UIService) resolveRouterNodeContext(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, nil
	}

	values := md.Get(routerAPIKeyHeader)
	if len(values) == 0 {
		return ctx, nil
	}

	apiKey := strings.TrimSpace(values[0])
	if apiKey == "" {
		return nil, status.Error(codes.Unauthenticated, "missing router api key")
	}

	router, err := s.db.GetRouterByAPIKey(apiKey)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, status.Error(codes.Unauthenticated, "invalid router api key")
		}
		return nil, status.Errorf(codes.Internal, "resolve router api key: %v", err)
	}

	return withResolvedNodeAddr(ctx, router.Addr), nil
}
