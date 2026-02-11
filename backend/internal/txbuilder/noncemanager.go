package txbuilder

import (
	"context"
	"errors"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

type NonceProvider interface {
	Next(ctx context.Context, addr common.Address) (uint64, error)
	Reset(addr common.Address)
}

type NonceManager struct {
	client ChainClient
	mu     sync.Mutex
	next   map[common.Address]uint64
}

func NewNonceManager(client ChainClient) *NonceManager {
	return &NonceManager{client: client, next: make(map[common.Address]uint64)}
}

func (m *NonceManager) Next(ctx context.Context, addr common.Address) (uint64, error) {
	if m.client == nil {
		return 0, errors.New("nonce manager client is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if n, ok := m.next[addr]; ok {
		m.next[addr] = n + 1
		return n, nil
	}
	nonce, err := m.client.PendingNonceAt(ctx, addr)
	if err != nil {
		return 0, err
	}
	m.next[addr] = nonce + 1
	return nonce, nil
}

func (m *NonceManager) Reset(addr common.Address) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.next, addr)
}
