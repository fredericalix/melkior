package redisstore

import (
	"context"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	store, err := New(mr.Addr(), "", 0)
	require.NoError(t, err)

	return store, mr
}

func TestCreateNode(t *testing.T) {
	store, mr := setupTestStore(t)
	defer mr.Close()
	defer store.Close()

	ctx := context.Background()
	node := &nodev1.Node{
		Name:   "test-node",
		Type:   nodev1.NodeType_VM,
		Status: nodev1.NodeStatus_UP,
		Labels: map[string]string{"env": "test"},
	}

	created, err := store.CreateNode(ctx, node)
	require.NoError(t, err)
	assert.NotEmpty(t, created.Id)
	assert.Equal(t, "test-node", created.Name)
	assert.NotNil(t, created.LastSeen)

	_, err = store.CreateNode(ctx, node)
	assert.Error(t, err)
}

func TestGetNode(t *testing.T) {
	store, mr := setupTestStore(t)
	defer mr.Close()
	defer store.Close()

	ctx := context.Background()
	node := &nodev1.Node{
		Name:   "test-node",
		Type:   nodev1.NodeType_VM,
		Status: nodev1.NodeStatus_UP,
	}

	created, err := store.CreateNode(ctx, node)
	require.NoError(t, err)

	retrieved, err := store.GetNode(ctx, created.Id)
	require.NoError(t, err)
	assert.Equal(t, created.Id, retrieved.Id)
	assert.Equal(t, created.Name, retrieved.Name)

	_, err = store.GetNode(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestUpdateNode(t *testing.T) {
	store, mr := setupTestStore(t)
	defer mr.Close()
	defer store.Close()

	ctx := context.Background()
	node := &nodev1.Node{
		Name:   "test-node",
		Type:   nodev1.NodeType_VM,
		Status: nodev1.NodeStatus_UP,
	}

	created, err := store.CreateNode(ctx, node)
	require.NoError(t, err)

	created.Name = "updated-node"
	created.Status = nodev1.NodeStatus_DOWN

	updated, err := store.UpdateNode(ctx, created)
	require.NoError(t, err)
	assert.Equal(t, "updated-node", updated.Name)
	assert.Equal(t, nodev1.NodeStatus_DOWN, updated.Status)
}

func TestUpdateStatus(t *testing.T) {
	store, mr := setupTestStore(t)
	defer mr.Close()
	defer store.Close()

	ctx := context.Background()
	node := &nodev1.Node{
		Name:   "test-node",
		Type:   nodev1.NodeType_VM,
		Status: nodev1.NodeStatus_UP,
	}

	created, err := store.CreateNode(ctx, node)
	require.NoError(t, err)

	updated, err := store.UpdateStatus(ctx, created.Id, nodev1.NodeStatus_DEGRADED)
	require.NoError(t, err)
	assert.Equal(t, nodev1.NodeStatus_DEGRADED, updated.Status)
}

func TestDeleteNode(t *testing.T) {
	store, mr := setupTestStore(t)
	defer mr.Close()
	defer store.Close()

	ctx := context.Background()
	node := &nodev1.Node{
		Name:   "test-node",
		Type:   nodev1.NodeType_VM,
		Status: nodev1.NodeStatus_UP,
	}

	created, err := store.CreateNode(ctx, node)
	require.NoError(t, err)

	err = store.DeleteNode(ctx, created.Id)
	require.NoError(t, err)

	_, err = store.GetNode(ctx, created.Id)
	assert.Error(t, err)
}

func TestListNodes(t *testing.T) {
	store, mr := setupTestStore(t)
	defer mr.Close()
	defer store.Close()

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		node := &nodev1.Node{
			Name:   fmt.Sprintf("node-%d", i),
			Type:   nodev1.NodeType_VM,
			Status: nodev1.NodeStatus_UP,
		}
		if i%2 == 0 {
			node.Type = nodev1.NodeType_BAREMETAL
		}
		_, err := store.CreateNode(ctx, node)
		require.NoError(t, err)
	}

	allNodes, err := store.ListNodes(ctx, 0, 0, 0, 0)
	require.NoError(t, err)
	assert.Len(t, allNodes, 5)

	vmNodes, err := store.ListNodes(ctx, nodev1.NodeType_VM, 0, 0, 0)
	require.NoError(t, err)
	assert.Len(t, vmNodes, 2)

	bareMetalNodes, err := store.ListNodes(ctx, nodev1.NodeType_BAREMETAL, 0, 0, 0)
	require.NoError(t, err)
	assert.Len(t, bareMetalNodes, 3)
}