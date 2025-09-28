package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
	"github.com/melkior/nodestatus/internal/events"
	"github.com/melkior/nodestatus/internal/redisstore"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NodeService struct {
	nodev1.UnimplementedNodeServiceServer
	store  *redisstore.Store
	broker *events.Broker
	logger *zap.Logger
}

func NewNodeService(store *redisstore.Store, broker *events.Broker, logger *zap.Logger) *NodeService {
	return &NodeService{
		store:  store,
		broker: broker,
		logger: logger,
	}
}

func (s *NodeService) CreateNode(ctx context.Context, req *nodev1.CreateNodeRequest) (*nodev1.CreateNodeResponse, error) {
	if req.Node == nil {
		return nil, status.Error(codes.InvalidArgument, "node is required")
	}

	if req.Node.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "node name is required")
	}

	if req.Node.Type == nodev1.NodeType_NODE_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "node type is required")
	}

	node, err := s.store.CreateNode(ctx, req.Node)
	if err != nil {
		s.logger.Error("failed to create node", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.logger.Info("node created",
		zap.String("id", node.Id),
		zap.String("name", node.Name),
		zap.String("type", node.Type.String()))

	s.broker.Publish(ctx, &nodev1.WatchEventsResponse{
		EventType: nodev1.EventType_CREATED,
		Node:      node,
	})

	return &nodev1.CreateNodeResponse{Node: node}, nil
}

func (s *NodeService) UpdateNode(ctx context.Context, req *nodev1.UpdateNodeRequest) (*nodev1.UpdateNodeResponse, error) {
	if req.Node == nil {
		return nil, status.Error(codes.InvalidArgument, "node is required")
	}

	if req.Node.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "node id is required")
	}

	node, err := s.store.UpdateNode(ctx, req.Node)
	if err != nil {
		s.logger.Error("failed to update node", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.logger.Info("node updated",
		zap.String("id", node.Id),
		zap.String("name", node.Name))

	s.broker.Publish(ctx, &nodev1.WatchEventsResponse{
		EventType: nodev1.EventType_UPDATED,
		Node:      node,
	})

	return &nodev1.UpdateNodeResponse{Node: node}, nil
}

func (s *NodeService) UpdateStatus(ctx context.Context, req *nodev1.UpdateStatusRequest) (*nodev1.UpdateStatusResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "node id is required")
	}

	if req.Status == nodev1.NodeStatus_NODE_STATUS_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "status is required")
	}

	node, err := s.store.UpdateStatus(ctx, req.Id, req.Status)
	if err != nil {
		s.logger.Error("failed to update node status", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.logger.Info("node status updated",
		zap.String("id", node.Id),
		zap.String("status", node.Status.String()))

	s.broker.Publish(ctx, &nodev1.WatchEventsResponse{
		EventType:     nodev1.EventType_UPDATED,
		Node:          node,
		ChangedFields: []string{"status"},
	})

	return &nodev1.UpdateStatusResponse{Node: node}, nil
}

func (s *NodeService) DeleteNode(ctx context.Context, req *nodev1.DeleteNodeRequest) (*nodev1.DeleteNodeResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "node id is required")
	}

	node, err := s.store.GetNode(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}

	if err := s.store.DeleteNode(ctx, req.Id); err != nil {
		s.logger.Error("failed to delete node", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.logger.Info("node deleted", zap.String("id", req.Id))

	s.broker.Publish(ctx, &nodev1.WatchEventsResponse{
		EventType: nodev1.EventType_DELETED,
		Node:      node,
	})

	return &nodev1.DeleteNodeResponse{Id: req.Id}, nil
}

func (s *NodeService) GetNode(ctx context.Context, req *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "node id is required")
	}

	node, err := s.store.GetNode(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}

	return &nodev1.GetNodeResponse{Node: node}, nil
}

func (s *NodeService) ListNodes(ctx context.Context, req *nodev1.ListNodesRequest) (*nodev1.ListNodesResponse, error) {
	pageSize := req.PageSize
	if pageSize == 0 {
		pageSize = 100
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	offset := 0
	if req.PageToken != "" {
		fmt.Sscanf(req.PageToken, "%d", &offset)
	}

	nodes, err := s.store.ListNodes(ctx, req.TypeFilter, req.StatusFilter, offset, int(pageSize))
	if err != nil {
		s.logger.Error("failed to list nodes", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	var nextPageToken string
	if len(nodes) == int(pageSize) {
		nextPageToken = fmt.Sprintf("%d", offset+int(pageSize))
	}

	return &nodev1.ListNodesResponse{
		Nodes:         nodes,
		NextPageToken: nextPageToken,
	}, nil
}

func (s *NodeService) WatchEvents(req *nodev1.WatchEventsRequest, stream nodev1.NodeService_WatchEventsServer) error {
	ctx := stream.Context()
	subID := uuid.New().String()
	sub := s.broker.Subscribe(subID)
	defer s.broker.Unsubscribe(subID)

	s.logger.Info("client subscribed to events", zap.String("subscriber_id", subID))

	lastID := "0"
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				events, err := s.store.GetEventStream(ctx, lastID)
				if err != nil {
					continue
				}

				for _, event := range events {
					node, _ := s.store.GetNode(ctx, event.NodeID)
					if node != nil {
						s.broker.Publish(ctx, &nodev1.WatchEventsResponse{
							EventType:     event.Type,
							Node:          node,
							ChangedFields: event.ChangedFields,
						})
					}
					lastID = event.ID
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("client disconnected from events", zap.String("subscriber_id", subID))
			return nil
		case event, ok := <-sub.Channel:
			if !ok {
				return nil
			}
			if err := stream.Send(event); err != nil {
				s.logger.Error("failed to send event", zap.Error(err))
				return err
			}
		}
	}
}