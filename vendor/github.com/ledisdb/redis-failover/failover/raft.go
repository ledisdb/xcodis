package failover

import (
	"encoding/json"
	"io"
	"os"
	"path"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/siddontang/go/log"
)

func (fsm *masterFSM) Apply(l *raft.Log) interface{} {
	var a action
	if err := json.Unmarshal(l.Data, &a); err != nil {
		log.Errorf("decode raft log err %v", err)
		return err
	}

	fsm.handleAction(&a)

	return nil
}

func (fsm *masterFSM) Snapshot() (raft.FSMSnapshot, error) {
	snap := new(masterSnapshot)
	snap.masters = make([]string, 0, len(fsm.masters))

	fsm.Lock()
	for master := range fsm.masters {
		snap.masters = append(snap.masters, master)
	}
	fsm.Unlock()
	return snap, nil
}

func (fsm *masterFSM) Restore(snap io.ReadCloser) error {
	defer snap.Close()

	d := json.NewDecoder(snap)
	var masters []string

	if err := d.Decode(&masters); err != nil {
		return err
	}

	fsm.Lock()
	for _, master := range masters {
		fsm.masters[master] = struct{}{}
	}
	fsm.Unlock()

	return nil
}

type masterSnapshot struct {
	masters []string
}

func (snap *masterSnapshot) Persist(sink raft.SnapshotSink) error {
	data, _ := json.Marshal(snap.masters)
	_, err := sink.Write(data)
	if err != nil {
		sink.Cancel()
	}
	return err
}

func (snap *masterSnapshot) Release() {

}

// redis-failover uses raft to elect the cluster leader and do monitoring and failover.
type Raft struct {
	r *raft.Raft

	log       *os.File
	dbStore   *raftboltdb.BoltStore
	trans     *raft.NetworkTransport
	peerStore *raft.JSONPeers

	raftAddr string
}

func newRaft(c *Config, fsm raft.FSM) (Cluster, error) {
	r := new(Raft)

	if len(c.Raft.Addr) == 0 {
		return nil, nil
	}

	peers := make([]string, 0, len(c.Raft.Cluster))

	r.raftAddr = c.Raft.Addr

	peers = raft.AddUniquePeer(peers, r.raftAddr)

	for _, cluster := range c.Raft.Cluster {
		peers = raft.AddUniquePeer(peers, cluster)
	}

	os.MkdirAll(c.Raft.DataDir, 0755)

	cfg := raft.DefaultConfig()

	if len(c.Raft.LogDir) == 0 {
		r.log = os.Stdout
	} else {
		os.MkdirAll(c.Raft.LogDir, 0755)
		logFile := path.Join(c.Raft.LogDir, "raft.log")
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
		if err != nil {
			return nil, err
		}
		r.log = f

		cfg.LogOutput = r.log
	}

	raftDBPath := path.Join(c.Raft.DataDir, "raft_db")
	var err error
	r.dbStore, err = raftboltdb.NewBoltStore(raftDBPath)
	if err != nil {
		return nil, err
	}

	fileStore, err := raft.NewFileSnapshotStore(c.Raft.DataDir, 1, r.log)
	if err != nil {
		return nil, err
	}

	r.trans, err = raft.NewTCPTransport(r.raftAddr, nil, 3, 5*time.Second, r.log)
	if err != nil {
		return nil, err
	}

	r.peerStore = raft.NewJSONPeers(c.Raft.DataDir, r.trans)

	if c.Raft.ClusterState == ClusterStateNew {
		log.Infof("cluster state is new, use new cluster config")
		r.peerStore.SetPeers(peers)
	} else {
		log.Infof("cluster state is existing, use previous + new cluster config")
		ps, err := r.peerStore.Peers()
		if err != nil {
			log.Errorf("get store peers error %v", err)
			return nil, err
		}

		for _, peer := range peers {
			ps = raft.AddUniquePeer(ps, peer)
		}

		r.peerStore.SetPeers(ps)
	}

	if peers, _ := r.peerStore.Peers(); len(peers) <= 1 {
		cfg.EnableSingleNode = true
		log.Warn("raft will run in single node mode, may only be used in test")
	}

	r.r, err = raft.NewRaft(cfg, fsm, r.dbStore, r.dbStore, fileStore, r.peerStore, r.trans)

	return r, err
}

func (r *Raft) Close() {
	if r.trans != nil {
		r.trans.Close()
	}

	if r.r != nil {
		future := r.r.Shutdown()
		if err := future.Error(); err != nil {
			log.Errorf("Error shutting down raft: %v", err)
		}
	}

	if r.dbStore != nil {
		r.dbStore.Close()
	}

	if r.log != os.Stdout {
		r.log.Close()
	}
}

func (r *Raft) AddMasters(addrs []string, timeout time.Duration) error {
	var a = action{
		Cmd:     addCmd,
		Masters: addrs,
	}

	return r.apply(&a, timeout)
}

func (r *Raft) DelMasters(addrs []string, timeout time.Duration) error {
	var a = action{
		Cmd:     delCmd,
		Masters: addrs,
	}

	return r.apply(&a, timeout)
}

func (r *Raft) SetMasters(addrs []string, timeout time.Duration) error {
	var a = action{
		Cmd:     setCmd,
		Masters: addrs,
	}

	return r.apply(&a, timeout)
}

func (r *Raft) AddPeer(peerAddr string) error {
	f := r.r.AddPeer(peerAddr)
	return f.Error()
}

func (r *Raft) DelPeer(peerAddr string) error {
	f := r.r.RemovePeer(peerAddr)
	return f.Error()

}

func (r *Raft) SetPeers(peerAddrs []string) error {
	f := r.r.SetPeers(peerAddrs)
	return f.Error()

}

func (r *Raft) GetPeers() ([]string, error) {
	peers, err := r.peerStore.Peers()
	if err != nil {
		return nil, err
	}

	addrs := make([]string, 0, len(peers))

	addrs = append(addrs, peers...)

	return addrs, nil
}

func (r *Raft) apply(a *action, timeout time.Duration) error {
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}

	f := r.r.Apply(data, timeout)
	return f.Error()
}

func (r *Raft) LeaderCh() <-chan bool {
	return r.r.LeaderCh()
}

func (r *Raft) IsLeader() bool {
	addr := r.r.Leader()
	if addr == "" {
		return false
	} else {
		return addr == r.raftAddr
	}
}

func (r *Raft) Leader() string {
	return r.r.Leader()
}

func (r *Raft) Barrier(timeout time.Duration) error {
	f := r.r.Barrier(timeout)
	return f.Error()
}
