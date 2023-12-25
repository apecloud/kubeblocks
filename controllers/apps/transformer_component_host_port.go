package apps

import (
	"fmt"
	"reflect"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
