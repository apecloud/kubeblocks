package protocol

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NodeCache interface {
	GetNode(ip string) (NodeClient, error)
}

type nodeCache struct {
	sync.RWMutex

	port  int64
	nodes map[string]NodeClient
}

func NewNodeCache(port int64) *nodeCache {
	return &nodeCache{
		port:  port,
		nodes: make(map[string]NodeClient),
	}
}

func (c *nodeCache) GetNode(ip string) (NodeClient, error) {
	var node NodeClient

	c.RLock()
	node, ok := c.nodes[ip]
	c.RUnlock()
	if ok {
		return node, nil
	}

	addr := fmt.Sprintf("%s:%d", ip, c.port)
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to dial: %v", addr)
	}
	node = NewNodeClient(conn)
	c.Lock()
	c.nodes[ip] = node
	c.Unlock()
	return node, nil
}
