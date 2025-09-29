package grpcclient

import (
	"context"
	"fmt"

	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
	"github.com/melkior/nodestatus/internal/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type Client struct {
	conn   *grpc.ClientConn
	client nodev1.NodeServiceClient
	token  string
}

func NewClient(addr, token string) (*Client, error) {
	logging.Debug("Creating gRPC client for %s", addr)

	logging.Debug("Calling grpc.NewClient...")
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logging.Error("grpc.NewClient failed: %v", err)
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	logging.Debug("grpc.NewClient successful, connection created")

	logging.Debug("Creating NodeServiceClient...")
	client := nodev1.NewNodeServiceClient(conn)

	logging.Debug("gRPC client created successfully for %s", addr)
	return &Client{
		conn:   conn,
		client: client,
		token:  token,
	}, nil
}

// Compatibility with new naming
func New(addr, token string) (*Client, error) {
	return NewClient(addr, token)
}

// NodeService returns the underlying node service client
func (c *Client) NodeService() nodev1.NodeServiceClient {
	return c.client
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) authContext(ctx context.Context) context.Context {
	if c.token != "" {
		return metadata.AppendToOutgoingContext(ctx, "authorization", fmt.Sprintf("Bearer %s", c.token))
	}
	return ctx
}

func (c *Client) CreateNode(ctx context.Context, node *nodev1.Node) (*nodev1.Node, error) {
	resp, err := c.client.CreateNode(c.authContext(ctx), &nodev1.CreateNodeRequest{Node: node})
	if err != nil {
		return nil, err
	}
	return resp.Node, nil
}

func (c *Client) UpdateNode(ctx context.Context, node *nodev1.Node) (*nodev1.Node, error) {
	resp, err := c.client.UpdateNode(c.authContext(ctx), &nodev1.UpdateNodeRequest{Node: node})
	if err != nil {
		return nil, err
	}
	return resp.Node, nil
}

func (c *Client) UpdateStatus(ctx context.Context, id string, status nodev1.NodeStatus) (*nodev1.Node, error) {
	resp, err := c.client.UpdateStatus(c.authContext(ctx), &nodev1.UpdateStatusRequest{
		Id:     id,
		Status: status,
	})
	if err != nil {
		return nil, err
	}
	return resp.Node, nil
}

func (c *Client) DeleteNode(ctx context.Context, id string) error {
	_, err := c.client.DeleteNode(c.authContext(ctx), &nodev1.DeleteNodeRequest{Id: id})
	return err
}

func (c *Client) GetNode(ctx context.Context, id string) (*nodev1.Node, error) {
	resp, err := c.client.GetNode(ctx, &nodev1.GetNodeRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return resp.Node, nil
}

func (c *Client) ListNodes(ctx context.Context, typeFilter nodev1.NodeType, statusFilter nodev1.NodeStatus) ([]*nodev1.Node, error) {
	var allNodes []*nodev1.Node
	pageToken := ""

	for {
		resp, err := c.client.ListNodes(ctx, &nodev1.ListNodesRequest{
			PageSize:     100,
			PageToken:    pageToken,
			TypeFilter:   typeFilter,
			StatusFilter: statusFilter,
		})
		if err != nil {
			return nil, err
		}

		allNodes = append(allNodes, resp.Nodes...)

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return allNodes, nil
}

func (c *Client) WatchEvents(ctx context.Context) (nodev1.NodeService_WatchEventsClient, error) {
	logging.Debug("Calling WatchEvents on gRPC client...")
	stream, err := c.client.WatchEvents(ctx, &nodev1.WatchEventsRequest{})
	if err != nil {
		logging.Error("WatchEvents failed: %v", err)
		return nil, err
	}
	logging.Debug("WatchEvents stream created successfully")
	return stream, nil
}