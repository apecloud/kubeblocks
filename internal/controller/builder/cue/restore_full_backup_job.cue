restoreJobName: string
component: {
	name: string
	podSpec: {
		containers: [...]
		volumes: [...]
	}
}
backup: {
	metadata: {
		name: string
		namespace: string
	}
	status: {
		persistentVolumeClaimName: string
		manifests: {
			backupTool: {
				filePath: string
			}
		}
	}
}
backupTool: {
	spec: {
		image: string
		physical: {
			restoreCommands: [string]
		}
		env: [...]
	}
}
pvcName: string
job: {
	apiVersion: "batch/v1"
  kind: "Job"
  metadata: {
  	name: restoreJobName
  	namespace: backup.metadata.namespace
  }
  spec: {
    template: {
      spec: {
      	restartPolicy: "Never"
      	containers: [
      		{
      			name: "restore"
            image: backupTool.spec.image
            command: ["sh", "-c"]
            args: backupTool.spec.physical.restoreCommands
            env: [
            	for _, v in backupTool.spec.env
            	{
            		v
            	},
            	{
            		name: "BACKUP_NAME",
            		value: backup.metadata.name
            	},
            	{
            		name: "BACKUP_DIR",
            		if backup.status.manifests.backupTool.filePath == _|_ {
            		  value: "/\(backup.metadata.name)/\(backup.metadata.namespace)",
            		}
            		if backup.status.manifests.backupTool.filePath != _|_ {
            		  value: "/\(backup.metadata.name)\(backup.status.manifests.backupTool.filePath)",
            		}
            	},
            ]
            resources: component.podSpec.containers[0].resources
            volumeMounts: [
            	for _, vt in component.volumeTypes
            	for _, vm in component.podSpec.containers[0].volumeMounts
            	if vt.type == "data" && vm.name == vt.name
            	{
            		vm
            	},
            	{
            		name: "\(component.name)-\(backup.status.persistentVolumeClaimName)"
            		mountPath: "/\(backup.metadata.name)"
            	}
            ]
            securityContext: {
            	allowPrivilegeEscalation: false
            	runAsUser: 0
            }
          }
        ]
        volumes: [
        	for _, vt in component.volumeTypes
          for _, vct in component.volumeClaimTemplates
        	if vt.type == "data" && vct.metadata.name == vt.name
          {
          	name: vct.metadata.name
          	persistentVolumeClaim: {
            	claimName: pvcName
            }
          },
          {
          	name: "\(component.name)-\(backup.status.persistentVolumeClaimName)"
          	persistentVolumeClaim: {
          		claimName: backup.status.persistentVolumeClaimName
          	}
          }
        ]
      }
    }
    backoffLimit: 4
  }
}