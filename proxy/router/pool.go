package router

import (
	"sync"

	"github.com/siddontang/goredis"
)

type serverPool struct {
	sync.RWMutex

	pools map[string]*goredis.Client
}

func newServerPool() *serverPool {
	p := new(serverPool)

	p.pools = make(map[string]*goredis.Client, 16)

	return p
}

func (p *serverPool) Close() {
	p.Lock()
	defer p.Unlock()

	for _, c := range p.pools {
		c.Close()
	}
}

func (p *serverPool) GetConn(server string) (*goredis.PoolConn, error) {
	p.RLock()
	c, ok := p.pools[server]
	p.RUnlock()

	if ok {
		return c.Get()
	} else {
		c = p.AddPool(server)
		return c.Get()
	}
}

func (p *serverPool) AddPool(server string) *goredis.Client {
	p.Lock()
	defer p.Unlock()
	c, ok := p.pools[server]
	if ok {
		return c
	}

	c = goredis.NewClient(server, "")

	// later use config?
	c.SetMaxIdleConns(16)
	c.SetReadBufferSize(4096)
	c.SetWriteBufferSize(4096)

	p.pools[server] = c
	return c
}
