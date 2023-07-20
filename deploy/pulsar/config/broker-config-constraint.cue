#PulsarBrokersParameter: {

	// Default number of message dispatching throttling-limit for every replicator in replication. Using a value of 0, is disabling replication message dispatch-throttling.
	dispatchThrottlingRatePerReplicatorInMsg: int & >= 0

	// Default number of message-bytes dispatching throttling-limit for a subscription. Using a value of 0, is disabling default message-byte dispatch-throttling.
	dispatchThrottlingRatePerSubscriptionInByte: int & >= 0

	...
}

configuration: #PulsarBrokersParameter & {
}
