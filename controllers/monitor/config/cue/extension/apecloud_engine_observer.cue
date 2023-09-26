parameters: {
	pod_observer: *"apecloud_k8s_observer" | string
  container_observer: *"runtime_container" | string
  scraper_config_file: *"/opt/oteld/etc/scraper.yaml" | string
}



output:
  apecloud_engine_observer: {
	  pod_observer: parameters.pod_observer
	  container_observer: parameters.container_observer
	  scraper_config_file: parameters.scraper_config_file
  }

