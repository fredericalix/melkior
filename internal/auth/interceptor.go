package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var mutatingMethods = map[string]bool{
	"/node.v1.NodeService/CreateNode":   true,
	"/node.v1.NodeService/UpdateNode":   true,
	"/node.v1.NodeService/UpdateStatus": true,
	"/node.v1.NodeService/DeleteNode":   true,
}

func UnaryAuthInterceptor(adminToken string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !mutatingMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		if err := validateToken(ctx, adminToken); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

func StreamAuthInterceptor(adminToken string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !mutatingMethods[info.FullMethod] {
			return handler(srv, ss)
		}

		if err := validateToken(ss.Context(), adminToken); err != nil {
			return err
		}

		return handler(srv, ss)
	}
}

func validateToken(ctx context.Context, expectedToken string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return status.Errorf(codes.Unauthenticated, "missing authorization header")
	}

	authHeader := values[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return status.Errorf(codes.Unauthenticated, "invalid authorization header format")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token != expectedToken {
		return status.Errorf(codes.PermissionDenied, "invalid token")
	}

	return nil
}