// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/siddontang/xcodis/utils"

	// "github.com/siddontang/xcodis/models"
	"github.com/siddontang/xcodis/proxy/router/topology"

	log "github.com/ngaut/logging"

	"github.com/juju/errors"
	topo "github.com/ngaut/go-zookeeper/zk"
	stats "github.com/ngaut/gostats"
)

var (
	OK_BYTES = []byte("+OK\r\n")
)

const (
	LedisBroker = "ledisdb"
	RedisBroker = "redis"
)

var (
	whiteTypeCommand = make(map[string]string)

	//for specail ledisdb commands
	whiteXCommand = make(map[string]struct{})
)

func init() {
	regTypeCmd := func(tp string, cmds ...string) {
		for _, cmd := range cmds {
			whiteTypeCommand[cmd] = tp
		}
	}

	regXCmd := func(cmds ...string) {
		for _, cmd := range cmds {
			whiteXCommand[cmd] = struct{}{}
		}
	}

	// for redis, we treat key and string commands as KV type too
	regTypeCmd("KV",
		"DECR",
		"DECRby",
		"DEL",
		"EXISTS",
		"GET",
		"GETSET",
		"INCR",
		"INCRBY",
		"MGET",
		"MSET",
		"SET",
		"SETNX",
		"SETEX",
		"EXPIRE",
		"EXPIREAT",
		"TTL",
		"PERSIST",
		"APPEND",
		"GETRANGE",
		"SETRANGE",
		"STRLEN",
		"BITCOUNT",
		"BITPOS",
		"GETBIT",
		"SETBIT")

	regTypeCmd("HASH",
		"HDEL",
		"HEXISTS",
		"HGET",
		"HGETALL",
		"HINCRBY",
		"HKEYS",
		"HLEN",
		"HMGET",
		"HMSET",
		"HSET",
		"HVALS",
		"HCLEAR",
		"HMCLEAR",
		"HEXPIRE",
		"HEXPIREAT",
		"HTTL",
		"HPERSIST",
		"HKEYEXISTS")

	regTypeCmd("LIST",
		"LINDEX",
		"LLEN",
		"LPOP",
		"LRANGE",
		"LPUSH",
		"RPOP",
		"RPUSH",
		"LCLEAR",
		"LMCLEAR",
		"LEXPIRE",
		"LEXPIREAT",
		"LTTL",
		"LPERSIST",
		"LKEYEXISTS")

	regTypeCmd("SET",
		"SADD",
		"SISMEMBER",
		"SMEMBERS",
		"SREM",
		"SCLEAR",
		"SMCLEAR",
		"SEXPIRE",
		"SEXPIREAT",
		"STTL",
		"SPERSIST",
		"SKEYEXISTS",

		//below, all keys would have same hash (maybe same tag).
		"SDIFF",
		"SDIFFSTORE",
		"SINTER",
		"SINTERSTORE",
		"SUNION",
		"SUNIONSTORE")

	regTypeCmd("ZSET",
		"ZADD",
		"ZCARD",
		"ZCOUNT",
		"ZINCRBY",
		"ZRANGE",
		"ZRANGEBYSCORE",
		"ZRANK",
		"ZREM",
		"ZREMRANGEBYRANK",
		"ZREMRANGEBYSCORE",
		"ZREVRANGE",
		"ZREVRANK",
		"ZREVRANGEBYSCORE",
		"ZRANGEBYLEX",
		"ZREMRANGEBYLEX",
		"ZLEXCOUNT",
		"ZCLEAR",
		"ZMCLEAR",
		"ZEXPIRE",
		"ZEXPIREAT",
		"ZTTL",
		"ZPERSIST",
		"ZKEYEXISTS",

		//below, all keys would have same hash (maybe same tag).
		"ZUNIONSTORE",
		"ZINTERSTORE")

	// below command, we can not know its type for the key,
	// so let ledisdb migrate all type datas for the key.
	regTypeCmd("ALL",
		"RESTORE")

	// below command is also supported, but should not be used in migration.
	regTypeCmd("SERVER",
		"PING",
		"QUIT",
		"SELECT",
		"AUTH",
		"ECHO")

	// for ledisdb, the first argument for some x prefix commands is the type
	regXCmd("XRESTORE", "XDUMP")
}

func isMulOp(op string) bool {
	switch op {
	case "MGET", "DEL", "MSET", "LMCLEAR", "HMCLEAR", "SMCLEAR", "ZMCLEAR":
		return true
	default:
		return false
	}
}

func validSlot(i int) bool {
	if i < 0 || i >= slot_num {
		return false
	}

	return true
}

func StringsContain(s []string, key string) bool {
	for _, val := range s {
		if val == key { //need our resopnse
			return true
		}
	}

	return false
}

func GetEventPath(evt interface{}) string {
	return evt.(topo.Event).Path
}

func CheckUlimit(min int) {
	ulimitN, err := exec.Command("/bin/sh", "-c", "ulimit -n").Output()
	if err != nil {
		log.Warning("get ulimit failed", err)
	}

	n, err := strconv.Atoi(strings.TrimSpace(string(ulimitN)))
	if err != nil || n < min {
		log.Fatalf("ulimit too small: %d, should be at least %d", n, min)
	}
}

func GetOriginError(err *errors.Err) error {
	if err != nil {
		if err.Cause() == nil && err.Underlying() == nil {
			return err
		} else {
			return err.Underlying()
		}
	}

	return err
}

func recordResponseTime(c *stats.Counters, d time.Duration) {
	switch {
	case d < 5:
		c.Add("0-5ms", 1)
	case d >= 5 && d < 10:
		c.Add("5-10ms", 1)
	case d >= 10 && d < 50:
		c.Add("10-50ms", 1)
	case d >= 50 && d < 200:
		c.Add("50-200ms", 1)
	case d >= 200 && d < 1000:
		c.Add("200-1000ms", 1)
	case d >= 1000 && d < 5000:
		c.Add("1000-5000ms", 1)
	case d >= 5000 && d < 10000:
		c.Add("5000-10000ms", 1)
	default:
		c.Add("10000ms+", 1)
	}
}

func checkMigrateKeys(op string, keys [][]byte) (int, [][]byte, error) {
	switch op {
	case "ZINTERSTORE", "ZUNIONSTORE", "EVAL", "EVALSHA", "SDIFF", "SDIFFSTORE",
		"SINTER", "SINTERSTORE", "SUNION", "SUNIONSTORE":
		slot, err := checkKeysInSameSlot(keys)
		return slot, keys, err
	default:
		//we will use the first key for migration
		return mapKey2Slot(keys[0]), keys[0:1], nil
	}
}

type Conf struct {
	proxyId     string
	productName string
	zkAddr      string
	f           topology.ZkFactory
	net_timeout int //seconds
	broker      string
	slot_num    int
}

func LoadConf(configFile string) (*Conf, error) {
	srvConf := &Conf{}
	conf, err := utils.InitConfigFromFile(configFile)
	if err != nil {
		log.Fatal(err)
	}

	srvConf.productName, _ = conf.ReadString("product", "test")
	if len(srvConf.productName) == 0 {
		log.Fatalf("invalid config: product entry is missing in %s", configFile)
	}
	srvConf.zkAddr, _ = conf.ReadString("zk", "")
	if len(srvConf.zkAddr) == 0 {
		log.Fatalf("invalid config: need zk entry is missing in %s", configFile)
	}
	srvConf.proxyId, _ = conf.ReadString("proxy_id", "")
	if len(srvConf.proxyId) == 0 {
		log.Fatalf("invalid config: need proxy_id entry is missing in %s", configFile)
	}

	srvConf.broker, _ = conf.ReadString("broker", "ledisdb")
	if len(srvConf.broker) == 0 {
		log.Fatalf("invalid config: need broker entry is missing in %s", configFile)
	}

	srvConf.slot_num, _ = conf.ReadInt("slot_num", 16)

	srvConf.net_timeout, _ = conf.ReadInt("net_timeout", 5)

	return srvConf, nil
}
