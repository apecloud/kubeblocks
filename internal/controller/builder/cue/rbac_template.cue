// Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
// This file is part of KubeBlocks project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

cluster: {
	metadata: {
		namespace: string
		name:      string
	}
	spec: {
		clusterDefinitionRef: string
	}
}

serviceaccount: {
	apiVersion: "v1"
	kind:       "ServiceAccount"
	metadata: {
		namespace: cluster.metadata.namespace
		name:      "kb-\(cluster.metadata.name)"
		labels: {
			"app.kubernetes.io/name":       cluster.spec.clusterDefinitionRef
			"app.kubernetes.io/instance":   cluster.metadata.name
			"app.kubernetes.io/managed-by": "kubeblocks"
		}
	}
}

rolebinding: {
	apiVersion: "rbac.authorization.k8s.io/v1"
	kind:       "RoleBinding"
	metadata: {
		name:      "kb-\(cluster.metadata.name)"
		namespace: cluster.metadata.namespace
		labels: {
			"app.kubernetes.io/name":       cluster.spec.clusterDefinitionRef
			"app.kubernetes.io/instance":   cluster.metadata.name
			"app.kubernetes.io/managed-by": "kubeblocks"
		}
	}
	roleRef: {
		apiGroup: "rbac.authorization.k8s.io"
		kind:     "ClusterRole"
		name:     "kubeblocks-cluster-pod-role"
	}
	subjects: [{
		kind:      "ServiceAccount"
		name:      "kb-\(cluster.metadata.name)"
		namespace: cluster.metadata.namespace
	}]
}

clusterrolebinding: {
	apiVersion: "rbac.authorization.k8s.io/v1"
	kind:       "ClusterRoleBinding"
	metadata: {
		name:      "kb-\(cluster.metadata.name)"
		labels: {
			"app.kubernetes.io/name":       cluster.spec.clusterDefinitionRef
			"app.kubernetes.io/instance":   cluster.metadata.name
			"app.kubernetes.io/managed-by": "kubeblocks"
		}
	}
	roleRef: {
		apiGroup: "rbac.authorization.k8s.io"
		kind:     "ClusterRole"
		name:     "kubeblocks-volume-protection-pod-role"
	}
	subjects: [{
		kind:      "ServiceAccount"
		name:      "kb-\(cluster.metadata.name)"
		namespace: cluster.metadata.namespace
	}]
}
