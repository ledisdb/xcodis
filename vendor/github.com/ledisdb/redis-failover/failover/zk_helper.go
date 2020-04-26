package failover

// copy from zkhelper and customize for our need

// zk helper functions
// modified from Vitess project

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/go-cloud/go-zookeeper/zk"
	"github.com/go-cloud/zkhelper"
	"github.com/siddontang/go/log"
)

// Experiment with a little bit of abstraction.
// FIMXE(msolo) This object may need a mutex to ensure it can be shared
// across goroutines.
type zMutex struct {
	mu          sync.Mutex
	zconn       zkhelper.Conn
	path        string // Path under which we try to create lock nodes.
	contents    string
	interrupted chan struct{}
	name        string // The name of the specific lock node we created.
	ephemeral   bool
	onRetryLock func()
}

// CreateMutex initializes an unaquired mutex. A mutex is released only
// by Unlock. You can clean up a mutex with delete, but you should be
// careful doing so.
func createMutex(zconn zkhelper.Conn, zkPath string, addr string) zkhelper.ZLocker {
	pid := os.Getpid()
	contents := fmt.Sprintf(`{"addr": "%v", "pid": %v}`, addr, pid)

	return &zMutex{zconn: zconn, path: zkPath, contents: contents, interrupted: make(chan struct{})}
}

// Interrupt releases a lock that's held.
func (zm *zMutex) Interrupt() {
	select {
	case zm.interrupted <- struct{}{}:
	default:
		log.Warnf("zmutex interrupt blocked")
	}
}

// Lock returns nil when the lock is acquired.
func (zm *zMutex) Lock(desc string) error {
	return zm.LockWithTimeout(365*24*time.Hour, desc)
}

// LockWithTimeout returns nil when the lock is acquired. A lock is
// held if the file exists and you are the creator. Setting the wait
// to zero makes this a nonblocking lock check.
//
// FIXME(msolo) Disallow non-super users from removing the lock?
func (zm *zMutex) LockWithTimeout(wait time.Duration, desc string) (err error) {
	timer := time.NewTimer(wait)
	defer func() {
		if panicErr := recover(); panicErr != nil || err != nil {
			zm.deleteLock()
		}
	}()
	// Ensure the rendezvous node is here.
	// FIXME(msolo) Assuming locks are contended, it will be cheaper to assume this just
	// exists.
	_, err = zkhelper.CreateRecursive(zm.zconn, zm.path, "", 0, zk.WorldACL(zkhelper.PERM_DIRECTORY))
	if err != nil && !zkhelper.ZkErrorEqual(err, zk.ErrNodeExists) {
		return err
	}

	lockPrefix := path.Join(zm.path, "lock-")
	zflags := zk.FlagSequence
	if zm.ephemeral {
		zflags = zflags | zk.FlagEphemeral
	}

	// update node content
	var lockContent map[string]interface{}
	err = json.Unmarshal([]byte(zm.contents), &lockContent)
	if err != nil {
		return err
	}
	lockContent["desc"] = desc
	newContent, err := json.Marshal(lockContent)
	if err != nil {
		return err
	}

createlock:
	lockCreated, err := zm.zconn.Create(lockPrefix, newContent, int32(zflags), zk.WorldACL(zkhelper.PERM_FILE))
	if err != nil {
		return err
	}
	name := path.Base(lockCreated)
	zm.mu.Lock()
	zm.name = name
	zm.mu.Unlock()

trylock:
	children, _, err := zm.zconn.Children(zm.path)
	if err != nil {
		return fmt.Errorf("zkutil: trylock failed %v", err)
	}
	sort.Strings(children)
	if len(children) == 0 {
		return fmt.Errorf("zkutil: empty lock: %v", zm.path)
	}

	if children[0] == name {
		// We are the lock owner.
		return nil
	}

	if zm.onRetryLock != nil {
		zm.onRetryLock()
	}

	// This is the degenerate case of a nonblocking lock check. It's not optimal, but
	// also probably not worth optimizing.
	if wait == 0 {
		return zkhelper.ErrTimeout
	}
	prevLock := ""
	for i := 1; i < len(children); i++ {
		if children[i] == name {
			prevLock = children[i-1]
			break
		}
	}
	if prevLock == "" {
		// This is an interesting case. The node disappeared
		// underneath us, probably due to a session loss. We can
		// recreate the lock node (with a new sequence number) and
		// keep trying.
		log.Warnf("zkutil: no lock node found: %v/%v", zm.path, zm.name)
		goto createlock
	}

	zkPrevLock := path.Join(zm.path, prevLock)
	exist, stat, watch, err := zm.zconn.ExistsW(zkPrevLock)
	if err != nil {
		// FIXME(msolo) Should this be a retry?
		return fmt.Errorf("zkutil: unable to watch previous lock node %v %v", zkPrevLock, err)
	}
	if stat == nil || !exist {
		goto trylock
	}
	select {
	case <-timer.C:
		return zkhelper.ErrTimeout
	case <-zm.interrupted:
		return zkhelper.ErrInterrupted
	case event := <-watch:
		log.Infof("zkutil: lock event: %v", event)
		// The precise event doesn't matter - try to read again regardless.
		goto trylock
	}
}

// Unlock returns nil if the lock was successfully
// released. Otherwise, it is most likely a zk related error.
func (zm *zMutex) Unlock() error {
	return zm.deleteLock()
}

func (zm *zMutex) deleteLock() error {
	zm.mu.Lock()
	zpath := path.Join(zm.path, zm.name)
	zm.mu.Unlock()

	err := zm.zconn.Delete(zpath, -1)
	if err != nil && !zkhelper.ZkErrorEqual(err, zk.ErrNoNode) {
		return err
	}
	return nil
}

// ZElector stores basic state for running an election.
type zElector struct {
	*zMutex
	path   string
	leader string
}

func (ze *zElector) isLeader() bool {
	return ze.leader == ze.name
}

type electionEvent struct {
	Event int
	Err   error
}

// CreateElection returns an initialized elector. An election is
// really a cycle of events. You are flip-flopping between leader and
// candidate. It's better to think of this as a stream of events that
// one needs to react to.
func createElection(zconn zkhelper.Conn, zkPath string, addr string, onRetryLock func()) zElector {
	zm := createMutex(zconn, path.Join(zkPath, "candidates"), addr).(*zMutex)
	zm.ephemeral = true
	zm.onRetryLock = onRetryLock
	return zElector{zMutex: zm, path: zkPath}
}

// RunTask returns nil when the underlyingtask ends or the error it
// generated.
func (ze *zElector) RunTask(task *electorTask) error {
	leaderPath := path.Join(ze.path, "leader")
	for {
		_, err := zkhelper.CreateRecursive(ze.zconn, leaderPath, "", 0, zk.WorldACL(zkhelper.PERM_FILE))
		if err == nil || zkhelper.ZkErrorEqual(err, zk.ErrNodeExists) {
			break
		}
		log.Warnf("election leader create failed: %v", err)
		time.Sleep(500 * time.Millisecond)
	}

	for {
		err := ze.Lock("RunTask")
		if err != nil {
			log.Warnf("election lock failed: %v", err)
			if err == zkhelper.ErrInterrupted {
				return zkhelper.ErrInterrupted
			}
			continue
		}
		// Confirm your win and deliver acceptance speech. This notifies
		// listeners who will have been watching the leader node for
		// changes.
		_, err = ze.zconn.Set(leaderPath, []byte(ze.contents), -1)
		if err != nil {
			log.Warnf("election promotion failed: %v", err)
			continue
		}

		log.Infof("election promote leader %v", leaderPath)
		taskErrChan := make(chan error)
		go func() {
			taskErrChan <- task.Run()
		}()

	watchLeader:
		// Watch the leader so we can get notified if something goes wrong.
		data, _, watch, err := ze.zconn.GetW(leaderPath)
		if err != nil {
			log.Warnf("election unable to watch leader node %v %v", leaderPath, err)
			// FIXME(msolo) Add delay
			goto watchLeader
		}

		if string(data) != ze.contents {
			log.Warnf("election unable to promote leader")
			task.Stop()
			// We won the election, but we didn't become the leader. How is that possible?
			// (see Bush v. Gore for some inspiration)
			// It means:
			//   1. Someone isn't playing by the election rules (a bad actor).
			//      Hard to detect - let's assume we don't have this problem. :)
			//   2. We lost our connection somehow and the ephemeral lock was cleared,
			//      allowing someone else to win the election.
			continue
		}

		// This is where we start our target process and watch for its failure.
	waitForEvent:
		select {
		case <-ze.interrupted:
			log.Warn("election interrupted - stop child process")
			task.Stop()
			// Once the process dies from the signal, this will all tear down.
			goto waitForEvent
		case taskErr := <-taskErrChan:
			// If our code fails, unlock to trigger an election.
			log.Infof("election child process ended: %v", taskErr)
			ze.Unlock()
			if task.Interrupted() {
				log.Warnf("election child process interrupted - stepping down")
				return zkhelper.ErrInterrupted
			}
			continue
		case zevent := <-watch:
			// We had a zk connection hiccup.  We have a few choices,
			// but it depends on the constraints and the events.
			//
			// If we get SESSION_EXPIRED our connection loss triggered an
			// election that we won't have won and the thus the lock was
			// automatically freed. We have no choice but to start over.
			if zevent.State == zk.StateExpired {
				log.Warnf("election leader watch expired")
				task.Stop()
				continue
			}

			// Otherwise, we had an intermittent issue or something touched
			// the node. Either we lost our position or someone broke
			// protocol and touched the leader node.  We just reconnect and
			// revalidate. In the meantime, assume we are still the leader
			// until we determine otherwise.
			//
			// On a reconnect we will be able to see the leader
			// information. If we still hold the position, great. If not, we
			// kill the associated process.
			//
			// On a leader node change, we need to perform the same
			// validation. It's possible an election completes without the
			// old leader realizing he is out of touch.
			log.Warnf("election leader watch event %v", zevent)
			goto watchLeader
		}
	}
}
