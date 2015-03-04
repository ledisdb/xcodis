package router

//for ledisdb

const (
	LedisBroker = "ledisdb"
)

func initWhiteListCommand() {
	regCmdType := func(tp string, cmds ...string) {
		for _, cmd := range cmds {
			whiteListCommand[cmd] = tp
		}
	}

	regCmdType("KV",
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

	regCmdType("HASH",
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

	regCmdType("LIST",
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

	regCmdType("SET",
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

	regCmdType("ZSET",
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
}

func getOpGroup(op string) string {
	s, ok := whiteListCommand[op]
	if !ok {
		return ""
	} else {
		return s
	}
}
