output:
  resource_attributes:
    pod: {
    	app_kubernetes_io_component: "`labels[\"app.kubernetes.io/component\"]`"
	    app_kubernetes_io_instance: "`labels[\"app.kubernetes.io/instance\"]`"
	    app_kubernetes_io_managed_by: "`labels[\"app.kubernetes.io/managed-by\"]`"
	    app_kubernetes_io_name: "`labels[\"app.kubernetes.io/name\"]`"
	    app_kubernetes_io_version: "`labels[\"app.kubernetes.io/version\"]`"
	    apps_kubeblocks_io_component_name: "`labels[\"apps.kubeblocks.io/component-name\"]`"
	    node: "${env:NODE_NAME}"
	    namespace: "`namespace`"
	    pod: "`name`"
	    job: "oteld-app"
    }
	  "k8s.node": {
	  	kubernetes_io_arch: "`labels[\"kubernetes.io/arch\"]`"
      kubernetes_io_hostname: "`labels[\"kubernetes.io/hostname\"]`"
      kubernetes_io_os: "`labels[\"kubernetes.io/os\"]`"
      node: "`name`"
      hostname: "`hostname`"
      job: "oteld-system"
	  }
