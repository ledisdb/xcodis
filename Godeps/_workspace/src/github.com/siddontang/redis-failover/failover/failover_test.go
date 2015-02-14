package failover

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/garyburd/redigo/redis"
	. "gopkg.in/check.v1"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

var zkAddr = flag.String("zk", "", "zookeeper address, seperated by comma")

func Test(t *testing.T) {
	TestingT(t)
}

type failoverTestSuite struct {
}

var _ = Suite(&failoverTestSuite{})

var testPort = []int{16379, 16380, 16381}

func (s *failoverTestSuite) SetUpSuite(c *C) {
	_, err := exec.LookPath("redis-server")
	c.Assert(err, IsNil)
}

func (s *failoverTestSuite) TearDownSuite(c *C) {

}

func (s *failoverTestSuite) SetUpTest(c *C) {
	for _, port := range testPort {
		s.stopRedis(c, port)
		s.startRedis(c, port)
		s.doCommand(c, port, "SLAVEOF", "NO", "ONE")
		s.doCommand(c, port, "FLUSHALL")
	}
}

func (s *failoverTestSuite) TearDownTest(c *C) {
	for _, port := range testPort {
		s.stopRedis(c, port)
	}
}

type redisChecker struct {
	sync.Mutex
	ok  bool
	buf bytes.Buffer
}

func (r *redisChecker) Write(data []byte) (int, error) {
	r.Lock()
	defer r.Unlock()

	r.buf.Write(data)
	if strings.Contains(r.buf.String(), "The server is now ready to accept connections") {
		r.ok = true
	}

	return len(data), nil
}

func (s *failoverTestSuite) startRedis(c *C, port int) {
	checker := &redisChecker{ok: false}
	// start redis and use memory only
	cmd := exec.Command("redis-server", "--port", fmt.Sprintf("%d", port), "--save", "")
	cmd.Stdout = checker
	cmd.Stderr = checker

	err := cmd.Start()
	c.Assert(err, IsNil)

	for i := 0; i < 20; i++ {
		var ok bool
		checker.Lock()
		ok = checker.ok
		checker.Unlock()

		if ok {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	c.Fatal("redis-server can not start ok after 10s")
}

func (s *failoverTestSuite) stopRedis(c *C, port int) {
	cmd := exec.Command("redis-cli", "-p", fmt.Sprintf("%d", port), "shutdown", "nosave")
	cmd.Run()
}

func (s *failoverTestSuite) doCommand(c *C, port int, cmd string, cmdArgs ...interface{}) interface{} {
	conn, err := redis.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	c.Assert(err, IsNil)

	v, err := conn.Do(cmd, cmdArgs...)
	c.Assert(err, IsNil)
	return v
}

func (s *failoverTestSuite) TestSimpleCheck(c *C) {
	cfg := new(Config)
	cfg.Addr = ":11000"

	port := testPort[0]
	cfg.Masters = []string{fmt.Sprintf("127.0.0.1:%d", port)}
	cfg.CheckInterval = 500
	cfg.MaxDownTime = 1

	app, err := NewApp(cfg)
	c.Assert(err, IsNil)

	defer app.Close()

	go func() {
		app.Run()
	}()

	s.doCommand(c, port, "SET", "a", 1)
	n, err := redis.Int(s.doCommand(c, port, "GET", "a"), nil)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	ch := s.addBeforeHandler(app)

	ms := app.masters.GetMasters()
	c.Assert(ms, DeepEquals, []string{fmt.Sprintf("127.0.0.1:%d", port)})

	s.stopRedis(c, port)

	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		c.Fatal("check is not ok after 5s, too slow")
	}
}

func (s *failoverTestSuite) TestFailoverCheck(c *C) {
	cfg := new(Config)
	cfg.Addr = ":11000"

	port := testPort[0]
	masterAddr := fmt.Sprintf("127.0.0.1:%d", port)

	cfg.Masters = []string{masterAddr}
	cfg.CheckInterval = 500
	cfg.MaxDownTime = 1

	app, err := NewApp(cfg)
	c.Assert(err, IsNil)

	defer app.Close()

	ch := s.addAfterHandler(app)

	go func() {
		app.Run()
	}()

	s.buildReplTopo(c)

	s.stopRedis(c, port)

	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		c.Fatal("failover is not ok after 5s, too slow")
	}
}

func (s *failoverTestSuite) TestOneFaftFailoverCheck(c *C) {
	s.testOneClusterFailoverCheck(c, "raft")
}

func (s *failoverTestSuite) checkLeader(c *C, apps []*App) *App {
	for i := 0; i < 20; i++ {
		for _, app := range apps {
			if app != nil && app.cluster.IsLeader() {
				return app
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	c.Assert(1, Equals, 0)

	return nil
}

func (s *failoverTestSuite) testOneClusterFailoverCheck(c *C, broker string) {
	apps := s.newClusterApp(c, 1, 0, broker)
	app := apps[0]

	defer app.Close()

	s.checkLeader(c, apps)

	port := testPort[0]
	masterAddr := fmt.Sprintf("127.0.0.1:%d", port)

	err := app.addMasters([]string{masterAddr})
	c.Assert(err, IsNil)

	ch := s.addBeforeHandler(app)

	ms := app.masters.GetMasters()
	c.Assert(ms, DeepEquals, []string{fmt.Sprintf("127.0.0.1:%d", port)})

	s.stopRedis(c, port)

	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		c.Fatal("check is not ok after 5s, too slow")
	}
}

func (s *failoverTestSuite) TestMultiRaftFailoverCheck(c *C) {
	s.testMultiClusterFailoverCheck(c, "raft")
}

func (s *failoverTestSuite) testMultiClusterFailoverCheck(c *C, broker string) {
	apps := s.newClusterApp(c, 3, 10, broker)
	defer func() {
		for _, app := range apps {
			app.Close()
		}
	}()

	// leader
	app := s.checkLeader(c, apps)

	port := testPort[0]
	masterAddr := fmt.Sprintf("127.0.0.1:%d", port)

	err := app.addMasters([]string{masterAddr})
	c.Assert(err, IsNil)

	ch := s.addBeforeHandler(app)

	ms := app.masters.GetMasters()
	c.Assert(ms, DeepEquals, []string{fmt.Sprintf("127.0.0.1:%d", port)})

	s.stopRedis(c, port)

	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		c.Fatal("check is not ok after 5s, too slow")
	}

	ms = app.masters.GetMasters()
	c.Assert(ms, DeepEquals, []string{})

	err = app.cluster.Barrier(5 * time.Second)
	c.Assert(err, IsNil)

	// close leader
	app.Close()

	// start redis
	s.startRedis(c, port)

	// wait other two elect new leader
	app = s.checkLeader(c, apps)

	err = app.addMasters([]string{masterAddr})
	c.Assert(err, IsNil)

	ch = s.addBeforeHandler(app)

	ms = app.masters.GetMasters()
	c.Assert(ms, DeepEquals, []string{fmt.Sprintf("127.0.0.1:%d", port)})

	s.stopRedis(c, port)

	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		c.Fatal("check is not ok after 5s, too slow")
	}
}

func (s *failoverTestSuite) TestOneZkFailoverCheck(c *C) {
	s.testOneClusterFailoverCheck(c, "zk")
}

func (s *failoverTestSuite) TestMultiZkFailoverCheck(c *C) {
	s.testMultiClusterFailoverCheck(c, "zk")
}

func (s *failoverTestSuite) addBeforeHandler(app *App) chan string {
	ch := make(chan string, 1)
	f := func(downMaster string) error {
		ch <- downMaster
		return nil
	}

	app.AddBeforeFailoverHandler(f)
	return ch
}

func (s *failoverTestSuite) addAfterHandler(app *App) chan string {
	ch := make(chan string, 1)
	f := func(oldMaster string, newMaster string) error {
		ch <- newMaster
		return nil
	}

	app.AddAfterFailoverHandler(f)
	return ch
}

func (s *failoverTestSuite) newClusterApp(c *C, num int, base int, broker string) []*App {
	port := 11000
	raftPort := 12000
	cluster := make([]string, 0, num)
	for i := 0; i < num; i++ {
		cluster = append(cluster, fmt.Sprintf("127.0.0.1:%d", raftPort+i+base))
	}
	apps := make([]*App, 0, num)

	for i := 0; i < num; i++ {
		cfg := new(Config)
		cfg.Broker = broker

		cfg.Addr = fmt.Sprintf(":%d", port+i)
		cfg.MaxDownTime = 1

		cfg.Raft.Addr = fmt.Sprintf("127.0.0.1:%d", raftPort+i+base)
		cfg.Raft.DataDir = fmt.Sprintf("./var/store/%d", i+base)
		cfg.Raft.LogDir = fmt.Sprintf("./var/log/%d", i+base)

		os.RemoveAll(cfg.Raft.DataDir)
		os.RemoveAll(cfg.Raft.LogDir)

		cfg.Raft.ClusterState = ClusterStateExisting
		cfg.Raft.Cluster = cluster

		if *zkAddr == "" {
			cfg.Zk.Addr = []string{"memory"}
		} else {
			cfg.Zk.Addr = strings.Split(*zkAddr, ",")
		}

		cfg.Zk.BaseDir = "/zk/redis/failover"

		app, err := NewApp(cfg)

		c.Assert(err, IsNil)
		go func() {
			app.Run()
		}()

		apps = append(apps, app)
	}

	return apps
}

func (s *failoverTestSuite) buildReplTopo(c *C) {
	port := testPort[0]

	s.doCommand(c, testPort[1], "SLAVEOF", "127.0.0.1", port)
	s.doCommand(c, testPort[2], "SLAVEOF", "127.0.0.1", port)

	s.doCommand(c, port, "SET", "a", 10)
	s.doCommand(c, port, "SET", "b", 20)

	s.waitReplConnected(c, testPort[1], 10)
	s.waitReplConnected(c, testPort[2], 10)

	s.waitSync(c, port, 10)

	n, err := redis.Int(s.doCommand(c, testPort[1], "GET", "a"), nil)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 10)

	n, err = redis.Int(s.doCommand(c, testPort[2], "GET", "a"), nil)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 10)
}

func (s *failoverTestSuite) waitReplConnected(c *C, port int, timeout int) {
	for i := 0; i < timeout*2; i++ {
		v, _ := redis.Values(s.doCommand(c, port, "ROLE"), nil)
		tp, _ := redis.String(v[0], nil)
		if tp == SlaveType {
			state, _ := redis.String(v[3], nil)
			if state == ConnectedState || state == SyncState {
				return
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	c.Fatalf("wait %ds, but 127.0.0.1:%d can not connect to master", timeout, port)
}

func (s *failoverTestSuite) waitSync(c *C, port int, timeout int) {
	g := newGroup(fmt.Sprintf("127.0.0.1:%d", port))

	for i := 0; i < timeout*2; i++ {
		err := g.doRole()
		c.Assert(err, IsNil)

		same := true
		offset := g.Master.Offset
		if offset > 0 {
			for _, slave := range g.Slaves {
				if slave.Offset != offset {
					same = false
				}
			}
		}

		if same {
			return
		}

		time.Sleep(500 * time.Millisecond)
	}

	c.Fatalf("wait %ds, but all slaves can not sync the same with master %v", timeout, g)
}
