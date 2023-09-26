parameters: {
	enable: *true | bool
  container_runtime_type: *"containerd" | string
}


output:
  runtime_contaienr: {
	enable: parameters.enable
  container_runtime_type: parameters.container_runtime_type
  }


