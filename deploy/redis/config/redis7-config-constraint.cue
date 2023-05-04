//Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
//This file is part of KubeBlocks project
//
//This program is free software: you can redistribute it and/or modify
//it under the terms of the GNU Affero General Public License as published by
//the Free Software Foundation, either version 3 of the License, or
//(at your option) any later version.
//
//This program is distributed in the hope that it will be useful
//but WITHOUT ANY WARRANTY; without even the implied warranty of
//MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//GNU Affero General Public License for more details.
//
//You should have received a copy of the GNU Affero General Public License
//along with this program.  If not, see <http://www.gnu.org/licenses/>.

#RedisParameter: {

	"acllog-max-len": int & >=1 & <=10000 | *128

	"acl-pubsub-default"?: string & "resetchannels" | "allchannels"

	activedefrag?: string & "yes" | "no"

	"active-defrag-cycle-max": int & >=1 & <=75 | *75

	"active-defrag-cycle-min": int & >=1 & <=75 | *5

	"active-defrag-ignore-bytes": int | *104857600

	"active-defrag-max-scan-fields": int & >=1 & <=1000000 | *1000

	"active-defrag-threshold-lower": int & >=1 & <=100 | *10

	"active-defrag-threshold-upper": int & >=1 & <=100 | *100

	"active-expire-effort": int & >=1 & <=10 | *1

	appendfsync?: string & "always" | "everysec" | "no"

	appendonly?: string & "yes" | "no"

	"client-output-buffer-limit-normal-hard-limit": int | *0

	"client-output-buffer-limit-normal-soft-limit": int | *0

	"client-output-buffer-limit-normal-soft-seconds": int | *0

	"client-output-buffer-limit-pubsub-hard-limit": int | *33554432

	"client-output-buffer-limit-pubsub-soft-limit": int | *8388608

	"client-output-buffer-limit-pubsub-soft-seconds": int | *60

	"client-output-buffer-limit-replica-soft-seconds": int | *60

	"client-query-buffer-limit": int & >=1048576 & <=1073741824 | *1073741824

	"close-on-replica-write"?: string & "yes" | "no"

	"cluster-allow-pubsubshard-when-down"?: string & "yes" | "no"

	"cluster-allow-reads-when-down"?: string & "yes" | "no"

	"cluster-enabled"?: string & "yes" | "no"

	"cluster-preferred-endpoint-type"?: string & "tls-dynamic" | "ip"

	"cluster-require-full-coverage"?: string & "yes" | "no"

	databases: int & >=1 & <=10000 | *16

	"hash-max-listpack-entries": int | *512

	"hash-max-listpack-value": int | *64

	"hll-sparse-max-bytes": int & >=1 & <=16000 | *3000

	"latency-tracking"?: string & "yes" | "no"

	"lazyfree-lazy-eviction"?: string & "yes" | "no"

	"lazyfree-lazy-expire"?: string & "yes" | "no"

	"lazyfree-lazy-server-del"?: string & "yes" | "no"

	"lazyfree-lazy-user-del"?: string & "yes" | "no"

	"lfu-decay-time": int | *1

	"lfu-log-factor": int | *10

	"list-compress-depth": int | *0

	"list-max-listpack-size": int | *-2

	"lua-time-limit": int & 5000 | *5000

	maxclients: int & >=1 & <=65000 | *65000

	"maxmemory-policy"?: string & "volatile-lru" | "allkeys-lru" | "volatile-lfu" | "allkeys-lfu" | "volatile-random" | "allkeys-random" | "volatile-ttl" | "noeviction"

	"maxmemory-samples": int | *3

	"min-replicas-max-lag": int | *10

	"min-replicas-to-write": int | *0

	"notify-keyspace-events"?: string

	"proto-max-bulk-len": int & >=1048576 & <=536870912 | *536870912

	"rename-commands"?: string & "APPEND" | "BITCOUNT" | "BITFIELD" | "BITOP" | "BITPOS" | "BLPOP" | "BRPOP" | "BRPOPLPUSH" | "BZPOPMIN" | "BZPOPMAX" | "CLIENT" | "COMMAND" | "DBSIZE" | "DECR" | "DECRBY" | "DEL" | "DISCARD" | "DUMP" | "ECHO" | "EVAL" | "EVALSHA" | "EXEC" | "EXISTS" | "EXPIRE" | "EXPIREAT" | "FLUSHALL" | "FLUSHDB" | "GEOADD" | "GEOHASH" | "GEOPOS" | "GEODIST" | "GEORADIUS" | "GEORADIUSBYMEMBER" | "GET" | "GETBIT" | "GETRANGE" | "GETSET" | "HDEL" | "HEXISTS" | "HGET" | "HGETALL" | "HINCRBY" | "HINCRBYFLOAT" | "HKEYS" | "HLEN" | "HMGET" | "HMSET" | "HSET" | "HSETNX" | "HSTRLEN" | "HVALS" | "INCR" | "INCRBY" | "INCRBYFLOAT" | "INFO" | "KEYS" | "LASTSAVE" | "LINDEX" | "LINSERT" | "LLEN" | "LPOP" | "LPUSH" | "LPUSHX" | "LRANGE" | "LREM" | "LSET" | "LTRIM" | "MEMORY" | "MGET" | "MONITOR" | "MOVE" | "MSET" | "MSETNX" | "MULTI" | "OBJECT" | "PERSIST" | "PEXPIRE" | "PEXPIREAT" | "PFADD" | "PFCOUNT" | "PFMERGE" | "PING" | "PSETEX" | "PSUBSCRIBE" | "PUBSUB" | "PTTL" | "PUBLISH" | "PUNSUBSCRIBE" | "RANDOMKEY" | "READONLY" | "READWRITE" | "RENAME" | "RENAMENX" | "RESTORE" | "ROLE" | "RPOP" | "RPOPLPUSH" | "RPUSH" | "RPUSHX" | "SADD" | "SCARD" | "SCRIPT" | "SDIFF" | "SDIFFSTORE" | "SELECT" | "SET" | "SETBIT" | "SETEX" | "SETNX" | "SETRANGE" | "SINTER" | "SINTERSTORE" | "SISMEMBER" | "SLOWLOG" | "SMEMBERS" | "SMOVE" | "SORT" | "SPOP" | "SRANDMEMBER" | "SREM" | "STRLEN" | "SUBSCRIBE" | "SUNION" | "SUNIONSTORE" | "SWAPDB" | "TIME" | "TOUCH" | "TTL" | "TYPE" | "UNSUBSCRIBE" | "UNLINK" | "UNWATCH" | "WAIT" | "WATCH" | "ZADD" | "ZCARD" | "ZCOUNT" | "ZINCRBY" | "ZINTERSTORE" | "ZLEXCOUNT" | "ZPOPMAX" | "ZPOPMIN" | "ZRANGE" | "ZRANGEBYLEX" | "ZREVRANGEBYLEX" | "ZRANGEBYSCORE" | "ZRANK" | "ZREM" | "ZREMRANGEBYLEX" | "ZREMRANGEBYRANK" | "ZREMRANGEBYSCORE" | "ZREVRANGE" | "ZREVRANGEBYSCORE" | "ZREVRANK" | "ZSCORE" | "ZUNIONSTORE" | "SCAN" | "SSCAN" | "HSCAN" | "ZSCAN" | "XINFO" | "XADD" | "XTRIM" | "XDEL" | "XRANGE" | "XREVRANGE" | "XLEN" | "XREAD" | "XGROUP" | "XREADGROUP" | "XACK" | "XCLAIM" | "XPENDING" | "GEORADIUS_RO" | "GEORADIUSBYMEMBER_RO" | "LOLWUT" | "XSETID" | "SUBSTR" | "BITFIELD_RO" | "ACL" | "STRALGO"

	"repl-backlog-size": int | *1048576

	"repl-backlog-ttl": int | *3600

	"replica-allow-chaining"?: string & "yes" | "no"

	"replica-ignore-maxmemory"?: string & "yes" | "no"

	"replica-lazy-flush"?: string & "yes" | "no"

	"reserved-memory-percent": int & >=0 & <=100 | *25

	"set-max-intset-entries": int & >=0 & <=500000000 | *512

	"slowlog-log-slower-than": int | *10000

	"slowlog-max-len": int | *128

	"stream-node-max-bytes": int | *4096

	"stream-node-max-entries": int | *100

	"tcp-keepalive": int | *300

	timeout: int | *0

	"tracking-table-max-keys": int & >=1 & <=100000000 | *1000000

	"zset-max-listpack-entries": int | *128

	"zset-max-listpack-value": int | *64

	"protected-mode"?: string & "yes" | "no"

	"enable-debug-command"?: string & "yes" | "no" | "local"

	...
}

configuration: #RedisParameter & {
}
