package datachannel

const (
	ExtraEnvChannelTopologyName = "KB_CHANNEL_TOPOLOGY_NAME"
	ExtraEnvChannelName         = "KB_CHANNEL_NAME"
	ExtraEnvChannelType         = "KB_CHANNEL_TYPE"

	ExtraEnvChannelRandomInt16 = "KB_CHANNEL_RANDOM_INT16"

	ExtraEnvSourceHostname = "KB_CHANNEL_SOURCE_HOSTNAME"
	ExtraEnvSourcePort     = "KB_CHANNEL_SOURCE_PORT"
	ExtraEnvSinkHostname   = "KB_CHANNEL_SINK_HOSTNAME"
	ExtraEnvSinkPort       = "KB_CHANNEL_SINK_PORT"

	ExtraEnvSourceUser     = "KB_CHANNEL_SOURCE_USER"
	ExtraEnvSourcePassword = "KB_CHANNEL_SOURCE_PASSWORD"
	ExtraEnvSinkUser       = "KB_CHANNEL_SINK_USER"
	ExtraEnvSinkPassword   = "KB_CHANNEL_SINK_PASSWORD"

	ExtraEnvRelyHubHostname = "KB_CHANNEL_RELY_HUB_%s_HOSTNAME"
	ExtraEnvRelyHubPort     = "KB_CHANNEL_RELY_HUB_%s_PORT"

	ExtraEnvUdf = "KB_CHANNEL_%s"
)

const (
	ChannelNameArg         = "channelName"
	ChannelTopologyNameArg = "channelTopologyName"
)
