parameters: {
	logLevel: *"debug" | string
  metricsPort: *6668 | int
}

output:
  telemetry: {
  	logs:
      level: parameters.logLevel
    metrics:
      address: "${env:HOST_IP}:" + "\(parameters.metricsPort)"
    resource:
      node: "${env:NODE_NAME}"
      job: "oteld-telemetry"
  }
  extensions: ["memory_ballast", "apecloud_k8s_observer", "apecloud_k8s_observer/node", "runtime_container", "apecloud_engine_observer"]