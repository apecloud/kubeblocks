#ZookeeperParameter: {
	// the length of a single tick, which is the basic time unit used by ZooKeeper, as measured in milliseconds. It is used to regulate heartbeats, and timeouts. For example, the minimum session timeout will be two ticks.
	"tickTime"?: int
	// The maximum time in ms that a message in any topic is kept in memory before flushed to disk. If not set, the value in log.flush.scheduler.interval.ms is used
	"log.flush.interval.ms"?: int

	// New in 3.4.0: The time interval in hours for which the purge task has to be triggered. Set to a positive integer (1 and above) to enable the auto purging. Defaults to 24.
	"autopurge.purge.interval"?: int

	"4lw.commands.whitelist"?: string
}