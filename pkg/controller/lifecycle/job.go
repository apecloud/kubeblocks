/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type job struct {
	namespace        string
	clusterName      string
	compName         string
	lifecycleActions *appsv1.ComponentLifecycleActions
	templateVars     map[string]any
}

var _ Lifecycle = &job{}

func (a *job) PostProvision(ctx context.Context, cli client.Reader, opts *Options) error {
	return ErrActionNotImplemented
}

func (a *job) PreTerminate(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &preTerminate{
		namespace:   a.namespace,
		clusterName: a.clusterName,
		compName:    a.compName,
		action:      a.lifecycleActions.PreTerminate,
	}
	_, err := a.checkedCallAction(ctx, cli, lfa.action, lfa, opts)
	return err
}

func (a *job) RoleProbe(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
	return nil, ErrActionNotImplemented
}

func (a *job) Switchover(ctx context.Context, cli client.Reader, opts *Options, candidate string) error {
	return ErrActionNotImplemented
}

func (a *job) MemberJoin(ctx context.Context, cli client.Reader, opts *Options) error {
	return ErrActionNotImplemented
}

func (a *job) MemberLeave(ctx context.Context, cli client.Reader, opts *Options) error {
	return ErrActionNotImplemented
}

func (a *job) AccountProvision(ctx context.Context, cli client.Reader, opts *Options, statement, user, password string) error {
	return ErrActionNotImplemented
}

func (a *job) checkedCallAction(ctx context.Context, cli client.Reader, spec *appsv1.Action, lfa lifecycleAction, opts *Options) ([]byte, error) {
	if spec == nil || spec.Exec == nil {
		return nil, errors.Wrap(ErrActionNotDefined, lfa.name())
	}
	if err := precondition(ctx, cli, a.namespace, a.clusterName, a.compName, spec); err != nil {
		return nil, err
	}
	// TODO: exactly once
	return a.callAction(ctx, cli, spec, lfa, opts)
}

func (a *job) callAction(ctx context.Context, cli client.Reader, spec *appsv1.Action, lfa lifecycleAction, opts *Options) ([]byte, error) {
	parameters, err := a.parameters(ctx, cli, lfa)
	if err != nil {
		return nil, err
	}
	req := &proto.ActionRequest{
		Action:     lfa.name(),
		Parameters: parameters,
	}
	return a.execActionInJob(ctx, spec, req)
}

func (a *job) parameters(ctx context.Context, cli client.Reader, lfa lifecycleAction) (map[string]string, error) {
	m, err := a.templateVarsParameters()
	if err != nil {
		return nil, err
	}
	sys, err := lfa.parameters(ctx, cli)
	if err != nil {
		return nil, err
	}

	for k, v := range sys {
		// template vars take precedence
		if _, ok := m[k]; !ok {
			m[k] = v
		}
	}
	return m, nil
}

func (a *job) templateVarsParameters() (map[string]string, error) {
	m := map[string]string{}
	for k, v := range a.templateVars {
		m[k] = v.(string)
	}
	return m, nil
}

func (a *job) execActionInJob(ctx context.Context, spec *appsv1.Action, req *proto.ActionRequest) ([]byte, error) {
	jobName := fmt.Sprintf("%s-%s-%s", a.clusterName, a.compName, req.Action)
	// Create job spec
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: a.namespace,
			Name:      jobName,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "action",
							Image:   spec.Exec.Image,
							Command: spec.Exec.Command,
							Args:    spec.Exec.Args,
							Env: []corev1.EnvVar{
								{
									Name:  "ACTION_REQUEST",
									Value: mustMarshalJSON(req),
								},
							},
						},
					},
				},
			},
		},
	}

	// Create the job
	if err := a.cli.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to create job: %v", err)
	}

	// Wait for job completion
	var jobResult []byte
	err := wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		if err := a.cli.Get(ctx, types.NamespacedName{Name: jobName, Namespace: a.namespace}, job); err != nil {
			return false, err
		}

		if job.Status.Succeeded > 0 {
			// Get logs from the pod
			pods := &corev1.PodList{}
			if err := a.cli.List(ctx, pods, client.InNamespace(a.namespace), client.MatchingLabels{"job-name": jobName}); err != nil {
				return false, err
			}

			if len(pods.Items) > 0 {
				pod := pods.Items[0]
				logs, err := a.getPodLogs(&pod)
				if err != nil {
					return false, err
				}
				jobResult = logs
				return true, nil
			}
		}

		if job.Status.Failed > 0 {
			return false, fmt.Errorf("job failed")
		}

		return false, nil
	})

	// Cleanup
	deletePolicy := metav1.DeletePropagationForeground
	if err := a.cli.Delete(ctx, job, &client.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		return nil, fmt.Errorf("failed to delete job: %v", err)
	}

	if err != nil {
		return nil, err
	}
	return jobResult, nil
}
