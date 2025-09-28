package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestUnaryAuthInterceptor(t *testing.T) {
	adminToken := "test-token"
	interceptor := UnaryAuthInterceptor(adminToken)

	tests := []struct {
		name      string
		method    string
		metadata  metadata.MD
		wantError bool
		wantCode  codes.Code
	}{
		{
			name:      "non-mutating method passes",
			method:    "/node.v1.NodeService/GetNode",
			metadata:  metadata.Pairs(),
			wantError: false,
		},
		{
			name:      "mutating method with valid token",
			method:    "/node.v1.NodeService/CreateNode",
			metadata:  metadata.Pairs("authorization", "Bearer test-token"),
			wantError: false,
		},
		{
			name:      "mutating method without metadata",
			method:    "/node.v1.NodeService/CreateNode",
			metadata:  nil,
			wantError: true,
			wantCode:  codes.Unauthenticated,
		},
		{
			name:      "mutating method with wrong token",
			method:    "/node.v1.NodeService/CreateNode",
			metadata:  metadata.Pairs("authorization", "Bearer wrong-token"),
			wantError: true,
			wantCode:  codes.PermissionDenied,
		},
		{
			name:      "mutating method with invalid header format",
			method:    "/node.v1.NodeService/CreateNode",
			metadata:  metadata.Pairs("authorization", "InvalidFormat"),
			wantError: true,
			wantCode:  codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.metadata != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.metadata)
			}

			info := &grpc.UnaryServerInfo{
				FullMethod: tt.method,
			}

			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				return "ok", nil
			}

			result, err := interceptor(ctx, nil, info, handler)

			if tt.wantError {
				assert.Error(t, err)
				st, ok := status.FromError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.wantCode, st.Code())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "ok", result)
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	expectedToken := "test-token"

	tests := []struct {
		name      string
		metadata  metadata.MD
		wantError bool
		wantCode  codes.Code
	}{
		{
			name:      "valid token",
			metadata:  metadata.Pairs("authorization", "Bearer test-token"),
			wantError: false,
		},
		{
			name:      "missing metadata",
			metadata:  nil,
			wantError: true,
			wantCode:  codes.Unauthenticated,
		},
		{
			name:      "missing authorization header",
			metadata:  metadata.Pairs(),
			wantError: true,
			wantCode:  codes.Unauthenticated,
		},
		{
			name:      "wrong token",
			metadata:  metadata.Pairs("authorization", "Bearer wrong-token"),
			wantError: true,
			wantCode:  codes.PermissionDenied,
		},
		{
			name:      "invalid format",
			metadata:  metadata.Pairs("authorization", "InvalidFormat"),
			wantError: true,
			wantCode:  codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.metadata != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.metadata)
			}

			err := validateToken(ctx, expectedToken)

			if tt.wantError {
				assert.Error(t, err)
				st, ok := status.FromError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.wantCode, st.Code())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}