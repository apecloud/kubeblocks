/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package action

import (
	"context"
	"fmt"
	"strings"

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
)

// CreateVolumeSnapshotAction is an action that creates the volume snapshot.
type CreateVolumeSnapshotAction struct {
	// Name is the Name of the action.
	Name string

	// Owner is the owner of the volume snapshot.
	Owner client.Object

	// ObjectMeta is the metadata of the volume snapshot.
	ObjectMeta metav1.ObjectMeta

	// PersistentVolumeClaimWrappers is the list of persistent volume claims wrapper to snapshot.
	PersistentVolumeClaimWrappers []PersistentVolumeClaimWrapper
}

type PersistentVolumeClaimWrapper struct {
	VolumeName            string
	PersistentVolumeClaim corev1.PersistentVolumeClaim
}

func NewPersistentVolumeClaimWrapper(pvc corev1.PersistentVolumeClaim, volumeName string) PersistentVolumeClaimWrapper {
	return PersistentVolumeClaimWrapper{PersistentVolumeClaim: pvc, VolumeName: volumeName}
}

var configVolumeSnapshotError = []string{
	"Failed to set default snapshot class with error",
	"Failed to get snapshot class with error",
	"Failed to create snapshot content with error cannot find CSI PersistentVolumeSource for volume",
}

func (c *CreateVolumeSnapshotAction) GetName() string {
	return c.Name
}

func (c *CreateVolumeSnapshotAction) Type() dpv1alpha1.ActionType {
	return dpv1alpha1.ActionTypeNone
}

func (c *CreateVolumeSnapshotAction) Execute(ctx Context) (*dpv1alpha1.ActionStatus, error) {
	sb := newStatusBuilder(c)
	handleErr := func(err error) (*dpv1alpha1.ActionStatus, error) {
		return sb.withErr(err).build(), err
	}

	if err := c.validate(); err != nil {
		return handleErr(err)
	}

	vsCli := intctrlutil.VolumeSnapshotCompatClient{
		Client: ctx.Client,
		Ctx:    ctx.Ctx,
	}

	var (
		ok   bool
		err  error
		snap *vsv1.VolumeSnapshot
	)
	for _, w := range c.PersistentVolumeClaimWrappers {
		key := client.ObjectKey{
			Namespace: w.PersistentVolumeClaim.Namespace,
			Name:      utils.GetBackupVolumeSnapshotName(c.ObjectMeta.Name, w.VolumeName),
		}
		// create volume snapshot
		if err = c.createVolumeSnapshotIfNotExist(ctx, vsCli, &w.PersistentVolumeClaim, key); err != nil {
			return handleErr(err)
		}

		ok, snap, err = ensureVolumeSnapshotReady(vsCli, key)
		if err != nil {
			return handleErr(err)
		}

		if !ok {
			return sb.startTimestamp(&snap.CreationTimestamp).build(), nil
		}
	}

	// volume snapshot is ready and status is not error
	// TODO(ldm): now only support one volume to take snapshot, set its time, size to status
	return sb.phase(dpv1alpha1.ActionPhaseCompleted).
		totalSize(snap.Status.RestoreSize.String()).
		timeRange(snap.Status.CreationTime, snap.Status.CreationTime).
		build(), nil
}

func (c *CreateVolumeSnapshotAction) validate() error {
	if len(c.PersistentVolumeClaimWrappers) == 0 {
		return errors.New("persistent volume claims are required")
	}
	if len(c.PersistentVolumeClaimWrappers) > 1 {
		return errors.New("only one persistent volume claim is supported")
	}
	return nil
}

// createVolumeSnapshotIfNotExist check volume snapshot exists, if not, create it.
func (c *CreateVolumeSnapshotAction) createVolumeSnapshotIfNotExist(ctx Context,
	vsCli intctrlutil.VolumeSnapshotCompatClient,
	pvc *corev1.PersistentVolumeClaim,
	key client.ObjectKey) error {
	var (
		err     error
		vscName string
	)

	snap := &vsv1.VolumeSnapshot{}
	exists, err := vsCli.CheckResourceExists(key, snap)
	if err != nil {
		return err
	}

	// if the volume snapshot already exists, skip creating it.
	if exists {
		return nil
	}

	c.ObjectMeta.Name = key.Name
	c.ObjectMeta.Namespace = key.Namespace

	// create volume snapshot
	snap = &vsv1.VolumeSnapshot{
		ObjectMeta: c.ObjectMeta,
		Spec: vsv1.VolumeSnapshotSpec{
			Source: vsv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &pvc.Name,
			},
		},
	}

	if vscName, err = c.getVolumeSnapshotClassName(ctx.Ctx, ctx.Client, vsCli, pvc.Spec.VolumeName); err != nil {
		return err
	}

	if vscName != "" {
		snap.Spec.VolumeSnapshotClassName = &vscName
	}

	controllerutil.AddFinalizer(snap, dptypes.DataProtectionFinalizerName)
	if err = utils.SetControllerReference(c.Owner, snap, ctx.Scheme); err != nil {
		return err
	}

	msg := fmt.Sprintf("creating volume snapshot %s/%s", snap.Namespace, snap.Name)
	ctx.Recorder.Event(c.Owner, corev1.EventTypeNormal, "CreatingVolumeSnapshot", msg)
	if err = vsCli.Create(snap); err != nil {
		return err
	}
	return nil
}

func (c *CreateVolumeSnapshotAction) getVolumeSnapshotClassName(
	ctx context.Context,
	cli client.Client,
	vsCli intctrlutil.VolumeSnapshotCompatClient,
	pvName string) (string, error) {
	pv := &corev1.PersistentVolume{}
	if err := cli.Get(ctx, types.NamespacedName{Name: pvName}, pv); err != nil {
		return "", err
	}
	if pv.Spec.CSI == nil {
		return "", nil
	}
	vscList := vsv1.VolumeSnapshotClassList{}
	if err := vsCli.List(&vscList); err != nil {
		return "", err
	}
	for _, item := range vscList.Items {
		if item.Driver == pv.Spec.CSI.Driver {
			return item.Name, nil
		}
	}
	return "", nil
}

func ensureVolumeSnapshotReady(
	vsCli intctrlutil.VolumeSnapshotCompatClient,
	key client.ObjectKey) (bool, *vsv1.VolumeSnapshot, error) {
	snap := &vsv1.VolumeSnapshot{}
	// not found, continue the creation process
	exists, err := vsCli.CheckResourceExists(key, snap)
	if err != nil {
		return false, nil, err
	}
	if exists && snap.Status != nil {
		// check if snapshot status throws an error, e.g. csi does not support volume snapshot
		if isVolumeSnapshotConfigError(snap) {
			return false, nil, errors.New(*snap.Status.Error.Message)
		}
		if boolptr.IsSetToTrue(snap.Status.ReadyToUse) {
			return true, snap, nil
		}
	}
	return false, snap, nil
}

func isVolumeSnapshotConfigError(snap *vsv1.VolumeSnapshot) bool {
	if snap.Status == nil || snap.Status.Error == nil || snap.Status.Error.Message == nil {
		return false
	}
	for _, errMsg := range configVolumeSnapshotError {
		if strings.Contains(*snap.Status.Error.Message, errMsg) {
			return true
		}
	}
	return false
}

var _ Action = &CreateVolumeSnapshotAction{}
