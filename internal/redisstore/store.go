package redisstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Store struct {
	client *redis.Client
}

func New(addr string, password string, db int) (*Store, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return &Store{client: client}, nil
}

func (s *Store) Close() error {
	return s.client.Close()
}

func (s *Store) CreateNode(ctx context.Context, node *nodev1.Node) (*nodev1.Node, error) {
	if node.Id == "" {
		node.Id = uuid.New().String()
	}

	if node.LastSeen == nil {
		node.LastSeen = timestamppb.Now()
	}

	existingID, err := s.client.Get(ctx, fmt.Sprintf("node:byname:%d:%s", node.Type, node.Name)).Result()
	if err == nil && existingID != "" {
		return nil, fmt.Errorf("node with name %s of type %s already exists", node.Name, node.Type.String())
	}

	if err := s.saveNode(ctx, node); err != nil {
		return nil, err
	}

	if err := s.appendEvent(ctx, nodev1.EventType_CREATED, node.Id, nil); err != nil {
		return nil, err
	}

	return node, nil
}

func (s *Store) UpdateNode(ctx context.Context, node *nodev1.Node) (*nodev1.Node, error) {
	oldNode, err := s.GetNode(ctx, node.Id)
	if err != nil {
		return nil, err
	}

	node.LastSeen = timestamppb.Now()

	changedFields := s.getChangedFields(oldNode, node)

	if err := s.deleteIndexes(ctx, oldNode); err != nil {
		return nil, err
	}

	if err := s.saveNode(ctx, node); err != nil {
		return nil, err
	}

	if err := s.appendEvent(ctx, nodev1.EventType_UPDATED, node.Id, changedFields); err != nil {
		return nil, err
	}

	return node, nil
}

func (s *Store) UpdateStatus(ctx context.Context, id string, status nodev1.NodeStatus) (*nodev1.Node, error) {
	node, err := s.GetNode(ctx, id)
	if err != nil {
		return nil, err
	}

	oldStatus := node.Status
	node.Status = status
	node.LastSeen = timestamppb.Now()

	if oldStatus != status {
		if err := s.deleteIndexes(ctx, node); err != nil {
			return nil, err
		}

		node.Status = status

		if err := s.saveNode(ctx, node); err != nil {
			return nil, err
		}

		if err := s.appendEvent(ctx, nodev1.EventType_UPDATED, node.Id, []string{"status"}); err != nil {
			return nil, err
		}
	}

	return node, nil
}

func (s *Store) DeleteNode(ctx context.Context, id string) error {
	node, err := s.GetNode(ctx, id)
	if err != nil {
		return err
	}

	if err := s.deleteIndexes(ctx, node); err != nil {
		return err
	}

	pipe := s.client.Pipeline()
	pipe.Del(ctx, fmt.Sprintf("node:%s", id))
	pipe.SRem(ctx, "nodes:all", id)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	if err := s.appendEvent(ctx, nodev1.EventType_DELETED, node.Id, nil); err != nil {
		return err
	}

	return nil
}

func (s *Store) GetNode(ctx context.Context, id string) (*nodev1.Node, error) {
	data, err := s.client.HGetAll(ctx, fmt.Sprintf("node:%s", id)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("node not found")
	}

	return s.nodeFromHash(data)
}

func (s *Store) ListNodes(ctx context.Context, typeFilter nodev1.NodeType, statusFilter nodev1.NodeStatus, offset, limit int) ([]*nodev1.Node, error) {
	var setKey string

	if typeFilter != nodev1.NodeType_NODE_TYPE_UNSPECIFIED && statusFilter != nodev1.NodeStatus_NODE_STATUS_UNSPECIFIED {
		typeSet := fmt.Sprintf("nodes:type:%d", typeFilter)
		statusSet := fmt.Sprintf("nodes:status:%d", statusFilter)
		setKey = fmt.Sprintf("temp:list:%d", time.Now().UnixNano())
		
		if err := s.client.ZInterStore(ctx, setKey, &redis.ZStore{
			Keys: []string{typeSet, statusSet},
		}).Err(); err != nil {
			return nil, fmt.Errorf("failed to intersect sets: %w", err)
		}
		defer s.client.Del(ctx, setKey)
	} else if typeFilter != nodev1.NodeType_NODE_TYPE_UNSPECIFIED {
		setKey = fmt.Sprintf("nodes:type:%d", typeFilter)
	} else if statusFilter != nodev1.NodeStatus_NODE_STATUS_UNSPECIFIED {
		setKey = fmt.Sprintf("nodes:status:%d", statusFilter)
	} else {
		setKey = "nodes:all"
	}

	members, err := s.client.SMembers(ctx, setKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	start := offset
	end := offset + limit
	if end > len(members) || limit == 0 {
		end = len(members)
	}
	if start >= len(members) {
		return []*nodev1.Node{}, nil
	}

	members = members[start:end]

	nodes := make([]*nodev1.Node, 0, len(members))
	for _, id := range members {
		node, err := s.GetNode(ctx, id)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (s *Store) GetEventStream(ctx context.Context, lastID string) ([]*Event, error) {
	args := &redis.XReadArgs{
		Streams: []string{"nodes:events", lastID},
		Count:   100,
		Block:   0,
	}

	if lastID == "" {
		args.Streams[1] = "$"
	}

	result, err := s.client.XRead(ctx, args).Result()
	if err != nil {
		return nil, err
	}

	var events []*Event
	for _, stream := range result {
		for _, msg := range stream.Messages {
			event, err := s.eventFromStreamMessage(msg)
			if err != nil {
				continue
			}
			events = append(events, event)
		}
	}

	return events, nil
}

type Event struct {
	ID            string
	Type          nodev1.EventType
	NodeID        string
	ChangedFields []string
	Timestamp     time.Time
}

func (s *Store) saveNode(ctx context.Context, node *nodev1.Node) error {
	labelsJSON, _ := json.Marshal(node.Labels)

	pipe := s.client.Pipeline()

	nodeKey := fmt.Sprintf("node:%s", node.Id)
	pipe.HSet(ctx, nodeKey, map[string]interface{}{
		"id":            node.Id,
		"type":          int32(node.Type),
		"name":          node.Name,
		"status":        int32(node.Status),
		"last_seen":     node.LastSeen.AsTime().Format(time.RFC3339),
		"labels_json":   string(labelsJSON),
		"metadata_json": node.MetadataJson,
	})

	pipe.Set(ctx, fmt.Sprintf("node:byname:%d:%s", node.Type, node.Name), node.Id, 0)

	pipe.SAdd(ctx, "nodes:all", node.Id)
	pipe.SAdd(ctx, fmt.Sprintf("nodes:type:%d", node.Type), node.Id)
	pipe.SAdd(ctx, fmt.Sprintf("nodes:status:%d", node.Status), node.Id)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to save node: %w", err)
	}

	return nil
}

func (s *Store) deleteIndexes(ctx context.Context, node *nodev1.Node) error {
	pipe := s.client.Pipeline()

	pipe.Del(ctx, fmt.Sprintf("node:byname:%d:%s", node.Type, node.Name))
	pipe.SRem(ctx, fmt.Sprintf("nodes:type:%d", node.Type), node.Id)
	pipe.SRem(ctx, fmt.Sprintf("nodes:status:%d", node.Status), node.Id)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete indexes: %w", err)
	}

	return nil
}

func (s *Store) appendEvent(ctx context.Context, eventType nodev1.EventType, nodeID string, changedFields []string) error {
	changedFieldsJSON, _ := json.Marshal(changedFields)

	args := &redis.XAddArgs{
		Stream: "nodes:events",
		Values: map[string]interface{}{
			"event_type":     int32(eventType),
			"node_id":        nodeID,
			"changed_fields": string(changedFieldsJSON),
			"ts":             time.Now().Unix(),
		},
	}

	if _, err := s.client.XAdd(ctx, args).Result(); err != nil {
		return fmt.Errorf("failed to append event: %w", err)
	}

	return nil
}

func (s *Store) nodeFromHash(data map[string]string) (*nodev1.Node, error) {
	node := &nodev1.Node{
		Id:           data["id"],
		Name:         data["name"],
		MetadataJson: data["metadata_json"],
	}

	var nodeType int32
	fmt.Sscanf(data["type"], "%d", &nodeType)
	node.Type = nodev1.NodeType(nodeType)

	var status int32
	fmt.Sscanf(data["status"], "%d", &status)
	node.Status = nodev1.NodeStatus(status)

	if lastSeenStr := data["last_seen"]; lastSeenStr != "" {
		if t, err := time.Parse(time.RFC3339, lastSeenStr); err == nil {
			node.LastSeen = timestamppb.New(t)
		}
	}

	if labelsJSON := data["labels_json"]; labelsJSON != "" {
		var labels map[string]string
		if err := json.Unmarshal([]byte(labelsJSON), &labels); err == nil {
			node.Labels = labels
		}
	}

	return node, nil
}

func (s *Store) eventFromStreamMessage(msg redis.XMessage) (*Event, error) {
	event := &Event{
		ID:     msg.ID,
		NodeID: msg.Values["node_id"].(string),
	}

	if eventTypeStr := msg.Values["event_type"].(string); eventTypeStr != "" {
		var eventType int32
		fmt.Sscanf(eventTypeStr, "%d", &eventType)
		event.Type = nodev1.EventType(eventType)
	}

	if changedFieldsStr := msg.Values["changed_fields"].(string); changedFieldsStr != "" {
		json.Unmarshal([]byte(changedFieldsStr), &event.ChangedFields)
	}

	if tsStr := msg.Values["ts"].(string); tsStr != "" {
		var ts int64
		fmt.Sscanf(tsStr, "%d", &ts)
		event.Timestamp = time.Unix(ts, 0)
	}

	return event, nil
}

func (s *Store) getChangedFields(old, new *nodev1.Node) []string {
	var fields []string

	if old.Name != new.Name {
		fields = append(fields, "name")
	}
	if old.Type != new.Type {
		fields = append(fields, "type")
	}
	if old.Status != new.Status {
		fields = append(fields, "status")
	}

	oldLabels, _ := json.Marshal(old.Labels)
	newLabels, _ := json.Marshal(new.Labels)
	if string(oldLabels) != string(newLabels) {
		fields = append(fields, "labels")
	}

	if old.MetadataJson != new.MetadataJson {
		fields = append(fields, "metadata_json")
	}

	return fields
}