
// required, options for command line input for args and flags.
options: {
    name: string
    namespace: string
    clusterDefRef: string
    appVersionRef: string
    components: [...]
    terminationPolicy: string
}

// required, k8s api resource content
content: {
    apiVersion: "dbaas.infracreate.com/v1alpha1"
    kind: "Cluster"
    metadata: {
        name: options.name
        namespace: options.namespace
    }
    spec: {
        clusterDefinitionRef: options.clusterDefRef
        appVersionRef: options.appVersionRef
        components: options.components
        terminationPolicy: options.terminationPolicy
    }
}
