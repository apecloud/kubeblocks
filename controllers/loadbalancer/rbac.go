package loadbalancer

// read + update access
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;update;watch

// read only + watch access
//+kubebuilder:rbac:groups=core,resources=pods;endpoints,verbs=get;list;watch

// read only access
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list
