package failover

import (
	"sync"
	"time"
)

type Cluster interface {
	Close()
	AddMasters(addrs []string, timeout time.Duration) error
	DelMasters(addrs []string, timeout time.Duration) error
	SetMasters(addrs []string, timeout time.Duration) error
	Barrier(timeout time.Duration) error
	IsLeader() bool
	LeaderCh() <-chan bool
}

// save mornitored master addr
type masterFSM struct {
	sync.Mutex

	masters map[string]struct{}
}

func newMasterFSM() *masterFSM {
	fsm := new(masterFSM)
	fsm.masters = make(map[string]struct{})
	return fsm
}

func (fsm *masterFSM) AddMasters(addrs []string) {
	fsm.Lock()
	defer fsm.Unlock()

	for _, addr := range addrs {
		if len(addr) == 0 {
			continue
		}
		fsm.masters[addr] = struct{}{}
	}

}

func (fsm *masterFSM) DelMasters(addrs []string) {
	fsm.Lock()
	defer fsm.Unlock()

	for _, addr := range addrs {
		if len(addr) == 0 {
			continue
		}

		delete(fsm.masters, addr)
	}
}

func (fsm *masterFSM) SetMasters(addrs []string) {
	m := make(map[string]struct{}, len(addrs))

	for _, addr := range addrs {
		if len(addr) == 0 {
			continue
		}

		m[addr] = struct{}{}
	}

	fsm.Lock()
	defer fsm.Unlock()

	fsm.masters = m
}

func (fsm *masterFSM) GetMasters() []string {
	fsm.Lock()
	defer fsm.Unlock()

	m := make([]string, 0, len(fsm.masters))
	for master := range fsm.masters {
		m = append(m, master)
	}

	return m
}

func (fsm *masterFSM) IsMaster(addr string) bool {
	fsm.Lock()
	defer fsm.Unlock()

	_, ok := fsm.masters[addr]
	return ok
}

func (fsm *masterFSM) Copy() *masterFSM {
	fsm.Lock()
	defer fsm.Unlock()

	o := new(masterFSM)
	o.masters = make(map[string]struct{}, len(fsm.masters))

	for master := range fsm.masters {
		o.masters[master] = struct{}{}
	}

	return o
}

const (
	addCmd = "add"
	delCmd = "del"
	setCmd = "set"
)

type action struct {
	Cmd     string   `json:"cmd"`
	Masters []string `json:"masters"`
}

func (fsm *masterFSM) handleAction(a *action) {
	switch a.Cmd {
	case addCmd:
		fsm.AddMasters(a.Masters)
	case delCmd:
		fsm.DelMasters(a.Masters)
	case setCmd:
		fsm.SetMasters(a.Masters)
	}
}
