package events

import (
	"context"
	"sync"

	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
)

type Subscriber struct {
	ID      string
	Channel chan *nodev1.WatchEventsResponse
}

type Broker struct {
	mu          sync.RWMutex
	subscribers map[string]*Subscriber
}

func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[string]*Subscriber),
	}
}

func (b *Broker) Subscribe(id string) *Subscriber {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub := &Subscriber{
		ID:      id,
		Channel: make(chan *nodev1.WatchEventsResponse, 100),
	}
	b.subscribers[id] = sub
	return sub
}

func (b *Broker) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if sub, exists := b.subscribers[id]; exists {
		close(sub.Channel)
		delete(b.subscribers, id)
	}
}

func (b *Broker) Publish(ctx context.Context, event *nodev1.WatchEventsResponse) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		select {
		case sub.Channel <- event:
		case <-ctx.Done():
			return
		default:
		}
	}
}

func (b *Broker) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}