//Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
//This file is part of KubeBlocks project
//
//This program is free software: you can redistribute it and/or modify
//it under the terms of the GNU Affero General Public License as published by
//the Free Software Foundation, either version 3 of the License, or
//(at your option) any later version.
//
//This program is distributed in the hope that it will be useful
//but WITHOUT ANY WARRANTY; without even the implied warranty of
//MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//GNU Affero General Public License for more details.
//
//You should have received a copy of the GNU Affero General Public License
//along with this program.  If not, see <http://www.gnu.org/licenses/>.

cluster: {
	metadata: {
		namespace: "default"
		name:      string
	}
}

sa: {
    apiVersion: "v1"
    kind: "ServiceAccount"
    metadata: {
		namespace: cluster.metadata.namespace
        name: "kb-addon-probe"
        labels: {
	      	"app.kubernetes.io/managed-by": "kubeblocks"
        }
    }
}


role: { 
    apiVersion: "rbac.authorization.k8s.io/v1"
    kind: "Role"
    metadata: {
        name: "kb-role-addon-probe"
        namespace: cluster.metadata.namespace
        labels: {
	        	"app.kubernetes.io/managed-by": "kubeblocks"
        }
    }
    rules: [{
        apiGroups: [""]
        resources: ["events"]
        verbs: ["create"]
    }]
}


rolebinding: {
    apiVersion: "rbac.authorization.k8s.io/v1"
    kind: "RoleBinding"
    metadata: {
        name: "kb-rolebinding-addon-probe"
        namespace: cluster.metadata.namespace
        labels: {
	        "app.kubernetes.io/managed-by": "kubeblocks"
        }
    }
    roleRef: {
        apiGroup: "rbac.authorization.k8s.io"
        kind: "Role"
        name: "kb-role-addon-probe"
        namespace: cluster.metadata.namespace
    }
    subjects: [{
        kind: "ServiceAccount"
        name: "kb-addon-probe"
        namespace: cluster.metadata.namespace
    }]
}
