#MongodParameter: {
	net: {
		port: int & >=0 & <=65535

		unixDomainSocket: {
			// Enables Unix Domain Sockets used for all network connections
			enabled:    bool | *false
			pathPrefix: string
			...
		}
		tls: {
			// Enables TLS used for all network connections
			mode: string & "disabled" | "allowTLS" | "preferTLS" | "requireTLS"

			certificateKeyFile: string
			CAFile:             string
			CRLFile:            string
			...
		}
		...
	}
	tls: {
		// Enables TLS used for all network connections
		mode: string & "disabled" | "allowTLS" | "preferTLS" | "requireTLS"

		certificateKeyFile: string
		CAFile:             string
		CRLFile:            string
		...
	}

	...
}

// configuration require
configuration: #MongodParameter & {
}
