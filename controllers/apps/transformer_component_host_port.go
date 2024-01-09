package apps

import (
	"fmt"
	"reflect"
	"strconv"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type componentHostPortTransformer struct {
}

var _ graph.Transformer = &componentHostPortTransformer{}

func (t *componentHostPortTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}
	synthesizeComp := transCtx.SynthesizeComponent
	if !synthesizeComp.PodSpec.HostNetwork {
		return nil
	}

	comp := transCtx.Component
	compObj := comp.DeepCopy()
	if err := buildContainerHostPorts(synthesizeComp, comp); err != nil {
		return err
	}

	if err := updateLorrySpecAfterPortsChanged(synthesizeComp); err != nil {
		return err
	}

	if reflect.DeepEqual(comp, compObj) {
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	graphCli.Update(dag, compObj, comp)
	return graph.ErrPrematureStop
}

func buildContainerHostPorts(synthesizeComp *component.SynthesizedComponent, comp *appsv1alpha1.Component) error {
	if !synthesizeComp.PodSpec.HostNetwork {
		return nil
	}

	pm := intctrlutil.GetPortManager()
	if comp.Annotations == nil {
		comp.Annotations = make(map[string]string)
	}
	comp.Annotations[constant.HostPortAnnotationKey] = "true"
	portMapping := make(map[int32]int32)
	for i, container := range synthesizeComp.PodSpec.Containers {
		for j, item := range container.Ports {
			portKey := intctrlutil.BuildHostPortName(synthesizeComp.ClusterName, synthesizeComp.Name, container.Name, item.Name)
			var (
				err  error
				port int32
			)
			if pm.NeedAllocate(item.ContainerPort) {
				port, err = pm.AllocatePort(portKey)
				if err != nil {
					return err
				}
				synthesizeComp.PodSpec.Containers[i].Ports[j].ContainerPort = port
			} else {
				if err = pm.UsePort(portKey, item.ContainerPort); err != nil {
					return err
				}
				port = item.ContainerPort
			}
			comp.Annotations[portKey] = fmt.Sprintf("%d", port)
			portMapping[item.ContainerPort] = port
		}
	}

	// update monitor scrape port
	if synthesizeComp.Monitor.Enable {
		newScrapePort, ok := portMapping[synthesizeComp.Monitor.ScrapePort]
		if !ok {
			return fmt.Errorf("monitor scrape port %d not found", synthesizeComp.Monitor.ScrapePort)
		}
		synthesizeComp.Monitor.ScrapePort = newScrapePort
	}
	return nil
}

func updateLorrySpecAfterPortsChanged(synthesizeComp *component.SynthesizedComponent) error {
	lorryContainer := intctrlutil.GetLorryContainer(synthesizeComp.PodSpec.Containers)
	if lorryContainer == nil {
		return nil
	}

	lorryHTTPPort := GetLorryHTTPPort(lorryContainer)
	lorryGRPCPort := GetLorryGRPCPort(lorryContainer)
	if err := updateLorry(synthesizeComp, lorryContainer, lorryHTTPPort, lorryGRPCPort); err != nil {
		return err
	}

	if err := updateReadinessProbe(synthesizeComp, lorryHTTPPort); err != nil {
		return err
	}
	return nil
}

func updateLorry(synthesizeComp *component.SynthesizedComponent, container *corev1.Container, httpPort, grpcPort int) error {
	container.Command = []string{"lorry",
		"--port", strconv.Itoa(httpPort),
		"--grpcport", strconv.Itoa(grpcPort),
	}

	if container.StartupProbe != nil && container.StartupProbe.TCPSocket != nil {
		container.StartupProbe.TCPSocket.Port = intstr.FromInt(httpPort)
	}

	for i := range container.Env {
		if container.Env[i].Name != constant.KBEnvServicePort {
			continue
		}
		if len(synthesizeComp.PodSpec.Containers) > 0 {
			mainContainer := synthesizeComp.PodSpec.Containers[0]
			if len(mainContainer.Ports) > 0 {
				port := mainContainer.Ports[0]
				dbPort := port.ContainerPort
				container.Env[i] = corev1.EnvVar{
					Name:      constant.KBEnvServicePort,
					Value:     strconv.Itoa(int(dbPort)),
					ValueFrom: nil,
				}
			}
		}
	}
	return nil
}

func updateReadinessProbe(synthesizeComp *component.SynthesizedComponent, lorryHTTPPort int) error {
	var container *corev1.Container
	for i := range synthesizeComp.PodSpec.Containers {
		container = &synthesizeComp.PodSpec.Containers[i]
		if container.ReadinessProbe == nil {
			continue
		}
		if container.ReadinessProbe.HTTPGet == nil {
			continue
		}
		if container.ReadinessProbe.HTTPGet.Path == constant.LorryRoleProbePath ||
			container.ReadinessProbe.HTTPGet.Path == constant.LorryVolumeProtectPath {
			container.ReadinessProbe.HTTPGet.Port = intstr.FromInt(lorryHTTPPort)
		}
	}
	return nil
}

func GetLorryHTTPPort(container *corev1.Container) int {
	for _, port := range container.Ports {
		if port.Name == constant.LorryHTTPPortName {
			return int(port.ContainerPort)
		}
	}
	return 0
}

func GetLorryGRPCPort(container *corev1.Container) int {
	for _, port := range container.Ports {
		if port.Name == constant.LorryGRPCPortName {
			return int(port.ContainerPort)
		}
	}
	return 0
}
