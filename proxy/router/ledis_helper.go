package router

//for ledisdb

const (
	LedisBroker = "ledisdb"
)

func initWhiteListCommand() {
	wc := whiteListCommand

	wc["DECR"] = "KV"
	wc["DECRby"] = "KV"
	wc["DEL"] = "KV"
	wc["EXISTS"] = "KV"
	wc["GET"] = "KV"
	wc["GETSET"] = "KV"
	wc["INCR"] = "KV"
	wc["INCRBY"] = "KV"
	wc["MGET"] = "KV"
	wc["MSET"] = "KV"
	wc["SET"] = "KV"
	wc["SETNX"] = "KV"
	wc["SETEX"] = "KV"
	wc["EXPIRE"] = "KV"
	wc["EXPIREAT"] = "KV"
	wc["TTL"] = "KV"
	wc["PERSIST"] = "KV"

	wc["HEDL"] = "HASH"
	wc["HEXISTS"] = "HASH"
	wc["HGET"] = "HASH"
	wc["HGETALL"] = "HASH"
	wc["HINCRBY"] = "HASH"
	wc["HKEYS"] = "HASH"
	wc["HLEN"] = "HASH"
	wc["HMGET"] = "HASH"
	wc["HMSET"] = "HASH"
	wc["HSET"] = "HASH"
	wc["HVALS"] = "HASH"
	wc["HCLEAR"] = "HASH"
	wc["HMCLEAR"] = "HASH"
	wc["HEXPIRE"] = "HASH"
	wc["HEXPIREAT"] = "HASH"
	wc["HTTL"] = "HASH"
	wc["HPERSIST"] = "HASH"

	wc["LINDEX"] = "LIST"
	wc["LLEN"] = "LIST"
	wc["LPOP"] = "LIST"
	wc["LRANGE"] = "LIST"
	wc["LPUSH"] = "LIST"
	wc["RPOP"] = "LIST"
	wc["RPUSH"] = "LIST"
	wc["LCLEAR"] = "LIST"
	wc["LMCLEAR"] = "LIST"
	wc["LEXPIRE"] = "LIST"
	wc["LEXPIREAT"] = "LIST"
	wc["LTTL"] = "LIST"
	wc["LPERSIST"] = "LIST"

	wc["SADD"] = "SET"
	wc["SISMEMBER"] = "SET"
	wc["SMEMBERS"] = "SET"
	wc["SREM"] = "SET"
	wc["SCLEAR"] = "SET"
	wc["SMCLEAR"] = "SET"
	wc["SEXPIRE"] = "SET"
	wc["SEXPIREAT"] = "SET"
	wc["STTL"] = "SET"
	wc["SPERSIST"] = "SET"

	wc["ZADD"] = "ZSET"
	wc["ZCARD"] = "ZSET"
	wc["ZCOUNT"] = "ZSET"
	wc["ZINCRBY"] = "ZSET"
	wc["ZRANGE"] = "ZSET"
	wc["ZRANGEBYSCORE"] = "ZSET"
	wc["ZRANK"] = "ZSET"
	wc["ZREM"] = "ZSET"
	wc["ZREMRANGEBYRANK"] = "ZSET"
	wc["ZREMRANGEBYSCORE"] = "ZSET"
	wc["ZREVRANGE"] = "ZSET"
	wc["ZREVRANK"] = "ZSET"
	wc["ZREVRANGEBYSCORE"] = "ZSET"
	wc["ZRANGEBYLEX"] = "ZSET"
	wc["ZREMRANGEBYLEX"] = "ZSET"
	wc["ZLEXCOUNT"] = "ZSET"
	wc["ZCLEAR"] = "ZSET"
	wc["ZMCLEAR"] = "ZSET"
	wc["ZEXPIRE"] = "ZSET"
	wc["ZEXPIREAT"] = "ZSET"
	wc["ZTTL"] = "ZSET"
	wc["ZPERSIST"] = "ZSET"

	//very dangerous, all keys would have same hash (maybe same tag).
	//we will use first key for hashing.
	wc["SDIFF"] = "SET"
	wc["SDIFFSTORE"] = "SET"
	wc["SINTER"] = "SET"
	wc["SINTERSTORE"] = "SET"
	wc["SUNION"] = "SET"
	wc["SUNIONSTORE"] = "SET"

	wc["ZUNIONSTORE"] = "ZSET"
	wc["ZINTERSTORE"] = "ZSET"
}

func getOpGroup(op string) string {
	s, ok := whiteListCommand[op]
	if !ok {
		return ""
	} else {
		return s
	}
}
