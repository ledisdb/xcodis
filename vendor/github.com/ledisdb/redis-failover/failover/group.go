package failover

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/siddontang/go/log"
	"github.com/siddontang/go/sync2"
)

var (
	ErrNodeDown    = errors.New("Node is down")
	ErrNodeAlive   = errors.New("Node may be still alive")
	ErrNoCandidate = errors.New("no proper candidate to be promoted to master")
	ErrNodeType    = errors.New("Node is not the expected type")
)

const (
	MasterType = "master"
	SlaveType  = "slave"
)

const (
	ConnectState    = "connect"
	ConnectingState = "connecting"
	ConnectedState  = "connected"
	SyncState       = "sync"
)

// A node represents a real redis server
type Node struct {
	// Redis address, only support tcp now
	Addr string

	// Replication offset
	Offset int64

	conn redis.Conn
}

func (n *Node) String() string {
	return fmt.Sprintf("{addr: %s offset: %d}", n.Addr, n.Offset)
}

func (n *Node) doCommand(cmd string, args ...interface{}) (interface{}, error) {
	var err error
	var v interface{}
	for i := 0; i < 3; i++ {
		if n.conn == nil {
			n.conn, err = redis.DialTimeout("tcp", n.Addr, 5*time.Second, 0, 0)
			if err != nil {
				log.Errorf("dial %s error: %v, try again", n.Addr, err)
				continue
			}

		}

		v, err = n.conn.Do(cmd, args...)
		if err != nil {
			log.Errorf("do %s command for %s error: %v, try again", cmd, n.Addr, err)
			n.conn.Close()
			n.conn = nil
			continue
		} else {
			return v, nil
		}
	}

	// go here means do command error, maybe redis is down.
	return nil, err
}

func (n *Node) doRole() ([]interface{}, error) {
	return redis.Values(n.doCommand("ROLE"))
}

func (n *Node) ping() error {
	_, err := n.doCommand("PING")
	return err
}

func (n *Node) slaveof(host string, port string) error {
	_, err := n.doCommand("SLAVEOF", host, port)
	return err
}

func (n *Node) doRelpInfo() (map[string]string, error) {
	v, err := redis.String(n.doCommand("INFO", "REPLICATION"))
	if err != nil {
		return nil, err
	}

	seps := strings.Split(v, "\r\n")
	// skip first line, is # Replication
	seps = seps[1:]

	m := make(map[string]string, len(seps))
	for _, s := range seps {
		kv := strings.SplitN(s, ":", 2)
		if len(kv) == 2 {
			m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return m, nil
}

func (n *Node) close() {
	if n.conn != nil {
		n.conn.Close()
		n.conn = nil
	}
}

// A group contains a Redis master and one or more slaves
// It will use role command per second to check master's alive
// and find slaves automatically.
type Group struct {
	Master *Node
	Slaves map[string]*Node

	CheckErrNum sync2.AtomicInt32

	m sync.Mutex
}

func newGroup(masterAddr string) *Group {
	g := new(Group)

	g.Master = &Node{Addr: masterAddr}
	g.Slaves = make(map[string]*Node)

	return g
}

func (g *Group) Close() {
	g.m.Lock()
	defer g.m.Unlock()

	g.Master.close()

	for _, slave := range g.Slaves {
		slave.close()
	}
}

func (g *Group) Check() error {
	g.m.Lock()
	defer g.m.Unlock()

	err := g.doRole()
	if err != nil {
		g.CheckErrNum.Add(1)
	} else {
		g.CheckErrNum.Set(0)
	}

	return err
}

func (g *Group) doRole() error {
	v, err := g.Master.doRole()
	if err != nil {
		return ErrNodeDown
	}

	// the first line is server type
	serverType, _ := redis.String(v[0], nil)
	if serverType != MasterType {
		log.Errorf("server %s is not master now", g.Master.Addr)
		return ErrNodeType
	}

	// second is master replication offset,
	g.Master.Offset, _ = redis.Int64(v[1], nil)

	// then slave list [host, port, offset]
	slaves, _ := redis.Values(v[2], nil)
	nodes := make(map[string]*Node, len(slaves))
	for i := 0; i < len(slaves); i++ {
		ss, _ := redis.Strings(slaves[i], nil)
		var n Node
		n.Addr = fmt.Sprintf("%s:%s", ss[0], ss[1])
		n.Offset, _ = strconv.ParseInt(fmt.Sprintf("%s", ss[2]), 10, 64)
		nodes[n.Addr] = &n
	}

	// we don't care slave add or remove too much, so only log
	for addr := range nodes {
		if _, ok := g.Slaves[addr]; !ok {
			log.Infof("slave %s added", addr)
		}
	}

	for addr, slave := range g.Slaves {
		if _, ok := nodes[addr]; !ok {
			log.Infof("slave %s removed", addr)
			slave.close()
		}
	}

	g.Slaves = nodes
	return nil
}

func (g *Group) Ping() error {
	g.m.Lock()
	defer g.m.Unlock()

	return g.Master.ping()
}

// Elect a best slave which has the most up-to-date data with master
func (g *Group) Elect() (string, error) {
	g.m.Lock()
	defer g.m.Unlock()

	var addr string
	var checkOffset int64 = 0
	var checkPriority int = 0

	for _, slave := range g.Slaves {
		m, err := slave.doRelpInfo()
		if err != nil {
			log.Infof("slave %s get replication info err %v, skip it", slave.Addr, err)
			continue
		}

		if m["slave"] == MasterType {
			log.Errorf("server %s is not slave now, skip it", slave.Addr)
			continue
		}

		if m["master_link_status"] == "up" {
			log.Infof("slave %s master_link_status is up, master %s may be not down???",
				slave.Addr, g.Master.Addr)
			return "", ErrNodeAlive
		}

		priority, _ := strconv.Atoi(m["slave_priority"])
		replOffset, _ := strconv.ParseInt(m["slave_repl_offset"], 10, 64)

		used := false
		// like redis-sentinel, first check priority, then salve repl offset
		if checkPriority < priority {
			used = true
		} else if checkPriority == priority {
			if checkOffset < replOffset {
				used = true
			}
		}

		if used {
			addr = slave.Addr
			checkPriority = priority
			checkOffset = replOffset
		}
	}

	if len(addr) == 0 {
		log.Errorf("no proper candidate to be promoted")
		return "", ErrNoCandidate
	}

	log.Infof("select slave %s as new master, priority:%d, repl_offset:%d", addr, checkPriority, checkOffset)

	return addr, nil
}

// Promote the slave to master, then let other slaves replicate from it
func (g *Group) Promote(addr string) error {
	g.m.Lock()
	defer g.m.Unlock()

	node := g.Slaves[addr]

	if err := node.slaveof("no", "one"); err != nil {
		return err
	}

	delete(g.Slaves, addr)

	g.Master = node

	host, port, _ := net.SplitHostPort(addr)
	for _, slave := range g.Slaves {
		if err := slave.slaveof(host, port); err != nil {
			// if we go here, the replication topology may be wrong
			// so use fatal level and we should fix it manually
			log.Fatalf("slaveof %s to master %s err %v", slave.Addr, addr, err)
		} else {
			log.Infof("slaveof %s to master %s ok", slave.Addr, addr)
		}
	}

	return nil
}
