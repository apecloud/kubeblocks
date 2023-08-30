#SystemLog: {
	destination:        string
	path:               string
	logAppend:          bool
	verbosity:          int
	quiet:              bool
	traceAllExceptions: bool
	syslogFacility:     string
	logRotate:          string
	timeStampFormat:    string

	component: #Component
	...
}

#Component: {
	...
}

#MongodbParameters: {
	systemLog: #SystemLog
}

configuration: #MongodbParameters & {
}
