/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package trace

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/testutil/k8s/mocks"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg     *rest.Config
	testEnv *envtest.Environment
	ctx     context.Context
	cancel  context.CancelFunc

	namespace       = "foo"
	name            = "bar"
	uid             = uuid.NewUUID()
	resourceVersion = "612345"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	initKBOwnershipRulesForTest(cfg)

	err = opsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	model.AddScheme(opsv1alpha1.AddToScheme)

	err = parametersv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	model.AddScheme(parametersv1alpha1.AddToScheme)

	err = kbappsv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	model.AddScheme(kbappsv1.AddToScheme)

	err = dpv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	model.AddScheme(dpv1alpha1.AddToScheme)

	err = snapshotv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	model.AddScheme(snapshotv1.AddToScheme)

	err = workloadsv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	model.AddScheme(workloadsv1.AddToScheme)

	err = tracev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	model.AddScheme(tracev1.AddToScheme)

	go func() {
		defer GinkgoRecover()
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func mockObjects(k8sMock *mocks.MockClient) (*kbappsv1.Cluster, []kbappsv1.Component) {
	primary := builder.NewClusterBuilder(namespace, name).SetUID(uid).SetResourceVersion(resourceVersion).GetObject()
	compNames := []string{"hello", "world"}
	var secondaries []kbappsv1.Component
	for _, compName := range compNames {
		fullCompName := fmt.Sprintf("%s-%s", primary.Name, compName)
		secondary := builder.NewComponentBuilder(namespace, fullCompName, "").
			SetOwnerReferences(kbappsv1.APIVersion, kbappsv1.ClusterKind, primary).
			SetUID(uid).
			GetObject()
		secondary.ResourceVersion = resourceVersion
		secondaries = append(secondaries, *secondary)
	}
	k8sMock.EXPECT().Scheme().Return(scheme.Scheme).AnyTimes()
	k8sMock.EXPECT().
		List(gomock.Any(), &kbappsv1.ComponentList{}, gomock.Any()).
		DoAndReturn(func(_ context.Context, list *kbappsv1.ComponentList, _ ...client.ListOption) error {
			list.Items = secondaries
			return nil
		}).Times(1)
	k8sMock.EXPECT().
		List(gomock.Any(), &corev1.ServiceList{}, gomock.Any()).
		DoAndReturn(func(_ context.Context, list *corev1.ServiceList, _ ...client.ListOption) error {
			return nil
		}).Times(1)
	k8sMock.EXPECT().
		List(gomock.Any(), &corev1.SecretList{}, gomock.Any()).
		DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
			return nil
		}).Times(1)
	componentSecondaries := []client.ObjectList{
		&workloadsv1.InstanceSetList{},
		&corev1.ServiceList{},
		&corev1.SecretList{},
		&corev1.ConfigMapList{},
		&corev1.PersistentVolumeClaimList{},
		&rbacv1.RoleBindingList{},
		&corev1.ServiceAccountList{},
		&batchv1.JobList{},
		&dpv1alpha1.BackupList{},
		&dpv1alpha1.RestoreList{},
		&parametersv1alpha1.ComponentParameterList{},
	}
	for _, secondary := range componentSecondaries {
		k8sMock.EXPECT().
			List(gomock.Any(), secondary, gomock.Any()).
			DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
				return nil
			}).Times(2)
	}
	return primary, secondaries
}
