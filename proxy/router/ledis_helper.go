package router

//for ledisdb

const (
	LedisBroker = "ledisdb"
)

//for ledisdb
var (
	whiteTypeCommand = make(map[string]string)
	whiteXCommand    = make(map[string]struct{})
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
