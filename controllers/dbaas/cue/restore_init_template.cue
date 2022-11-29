restoreInitContainer: {
	name: "restore"
	command: ["sh", "-c"]
	args: ["[[ $(ls -A ${DATA_DIR}) ]] && exit 0;"]
	imagePullPolicy: "IfNotPresent"
	securityContext: {
		allowPrivilegeEscalation: false
		runAsUser:                0
	}
}
