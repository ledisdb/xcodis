package failover

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-cloud/zkhelper"
	"github.com/siddontang/go/log"
	"github.com/siddontang/go/sync2"
)

type zkAction struct {
	a       *action
	ch      chan error
	timeout sync2.AtomicBool
}

type Zk struct {
	m sync.Mutex

	c    *Config
	conn zkhelper.Conn
	fsm  *masterFSM

	elector  zElector
	isLeader sync2.AtomicBool

	leaderCh chan bool

	closed bool
	quit   chan struct{}

	actionCh chan *zkAction

	wg sync.WaitGroup
}

func newZk(cfg *Config, fsm *masterFSM) (Cluster, error) {
	z := new(Zk)

	var err error

	if !strings.HasPrefix(cfg.Zk.BaseDir, "/zk") {
		return nil, fmt.Errorf("invalid zk base dir %s, must have prefix /zk", cfg.Zk.BaseDir)
	}

	addr := strings.Join(cfg.Zk.Addr, ",")
	if addr == "memory" {
		// only for test
		log.Infof("only for test, use memory")
		z.conn = zkhelper.NewConn()
	} else {
		z.conn, err = zkhelper.ConnectToZk(addr)
	}

	if err != nil {
		return nil, err
	}

	z.c = cfg
	z.fsm = fsm
	z.isLeader.Set(false)
	z.leaderCh = make(chan bool, 1)
	z.actionCh = make(chan *zkAction, 10)

	z.quit = make(chan struct{})

	if _, err = zkhelper.CreateOrUpdate(z.conn, cfg.Zk.BaseDir, "", 0, zkhelper.DefaultDirACLs(), true); err != nil {
		log.Errorf("create %s error: %v", cfg.Zk.BaseDir, err)
		return nil, err
	}

	onRetryLock := func() {
		z.noticeLeaderCh(false)
	}

	z.elector = createElection(z.conn, cfg.Zk.BaseDir, cfg.Addr, onRetryLock)

	z.checkLeader()

	return z, nil
}

func (z *Zk) Close() {
	z.m.Lock()
	defer z.m.Unlock()

	if z.isClosed() {
		return
	}

	close(z.quit)

	z.conn.Close()

	z.wg.Wait()
}

func (z *Zk) IsLeader() bool {
	return z.isLeader.Get()
}

func (z *Zk) AddMasters(addrs []string, timeout time.Duration) error {
	var a = action{
		Cmd:     addCmd,
		Masters: addrs,
	}

	return z.apply(&a, timeout)
}

func (z *Zk) DelMasters(addrs []string, timeout time.Duration) error {
	var a = action{
		Cmd:     delCmd,
		Masters: addrs,
	}

	return z.apply(&a, timeout)
}

func (z *Zk) SetMasters(addrs []string, timeout time.Duration) error {
	var a = action{
		Cmd:     setCmd,
		Masters: addrs,
	}

	return z.apply(&a, timeout)
}

func (z *Zk) apply(a *action, timeout time.Duration) error {
	if !z.IsLeader() {
		return fmt.Errorf("node is not leader now")
	}

	act := &zkAction{
		a:  a,
		ch: make(chan error, 1),
	}
	act.timeout.Set(false)

	z.actionCh <- act

	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	select {
	case err := <-act.ch:
		return err
	case <-time.After(timeout):
		act.timeout.Set(true)
		return fmt.Errorf("handle action timeout, after %s", timeout)
	}

}

func (z *Zk) Barrier(timeout time.Duration) error {
	return nil
}

func (z *Zk) LeaderCh() <-chan bool {
	return z.leaderCh
}

func (z *Zk) noticeLeaderCh(b bool) {
	z.isLeader.Set(b)

	for {
		select {
		case z.leaderCh <- b:
			return
		default:
			log.Warnf("%s leader chan blocked, leader: %v", z.c.Addr, b)
			select {
			case <-z.leaderCh:
			default:
			}
		}
	}
}

func (z *Zk) isClosed() bool {
	select {
	case <-z.quit:
		return true
	default:
		return false
	}
}

func (z *Zk) checkLeader() {
	task := new(electorTask)
	task.z = z
	task.interrupted.Set(false)
	task.stop = make(chan struct{})

	go func() {
		err := z.elector.RunTask(task)
		if err != nil {
			log.Errorf("run elector task err: %v", err)
		}
	}()
}

func (z *Zk) getMasters() error {
	zkPath := fmt.Sprintf("%s/masters", z.c.Zk.BaseDir)

	exists, _, err := z.conn.Exists(zkPath)
	if err != nil {
		return err
	} else if !exists {
		if _, err = z.conn.Create(zkPath, nil, 0, zkhelper.DefaultFileACLs()); err != nil {
			return err
		}
	}

	data, _, err := z.conn.Get(zkPath)
	if err != nil {
		return err
	}

	if len(data) > 0 {
		var masters []string
		if err = json.Unmarshal(data, &masters); err != nil {
			return err
		}

		z.fsm.SetMasters(masters)
	}
	return nil
}

func (z *Zk) handleAction(a *action) error {
	log.Infof("handle action %s, masters: %v", a.Cmd, a.Masters)

	m := z.fsm.Copy()

	m.handleAction(a)

	masters := m.GetMasters()
	data, _ := json.Marshal(masters)

	zkPath := fmt.Sprintf("%s/masters", z.c.Zk.BaseDir)

	_, err := z.conn.Set(zkPath, data, -1)
	if err != nil {
		return err
	}

	z.fsm.SetMasters(masters)
	return nil
}

type electorTask struct {
	z *Zk

	interrupted sync2.AtomicBool

	stop chan struct{}
}

func (t *electorTask) Run() error {
	t.z.wg.Add(1)
	defer t.z.wg.Done()

	log.Infof("begin leader %s, run", t.z.c.Addr)

	if err := t.z.getMasters(); err != nil {
		t.interrupted.Set(true)

		log.Errorf("get masters err %v", err)
		return err
	}

	t.z.noticeLeaderCh(true)

	for {
		select {
		case <-t.z.quit:
			log.Info("zk close, interrupt elector running task")

			t.z.noticeLeaderCh(false)

			t.interrupted.Set(true)
			return nil
		case <-t.stop:
			log.Info("stop elector running task")
			return nil
		case a := <-t.z.actionCh:
			if a.timeout.Get() {
				log.Warnf("wait action %s masters %v timeout, skip it", a.a.Cmd, a.a.Masters)
			} else {
				err := t.z.handleAction(a.a)

				a.ch <- err
			}
		}
	}
}

func (t *electorTask) Stop() {
	t.z.noticeLeaderCh(false)

	t.interrupted.Set(false)

	select {
	case t.stop <- struct{}{}:
	default:
		log.Warnf("stop chan blocked")
	}
}

func (t *electorTask) Interrupted() bool {
	return t.interrupted.Get()
}
