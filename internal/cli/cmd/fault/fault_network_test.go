package fault

import (
	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("Fault Network", func() {
	var (
		tf      *cmdtesting.TestFactory
		streams genericclioptions.IOStreams
	)
	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Context("test fault network", func() {
		It("fault network partition", func() {
			inputs := [][]string{
				{""},
				{"--mode=one", "--dry-run=client"},
				{"--mode=fixed", "--value=2", "--dry-run=client"},
				{"--mode=fixed-percent", "--value=50", "--dry-run=client"},
				{"--mode=random-max-percent", "--value=50", "--dry-run=client"},
				{"--ns-fault=kb-system", "--dry-run=client"},
				{"--node=minikube-m02", "--dry-run=client"},
				{"--label=app.kubernetes.io/component=mysql", "--dry-run=client"},
				{"--node-label=kubernetes.io/arch=arm64", "--dry-run=client"},
				{"--annotation=example-annotation=group-a", "--dry-run=client"},
				{"--external-target=www.baidu.com", "--dry-run=client"},
				{"--target-mode=one", "--target-label=statefulset.kubernetes.io/pod-name=mycluster-mysql-2", "--target-ns-fault=default", "--dry-run=client"},
			}
			o := NewNetworkChaosOptions(tf, streams, string(v1alpha1.PartitionAction))
			cmd := o.NewCobraCommand(Partition, PartitionShort)
			o.AddCommonFlag(cmd)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault network loss", func() {
			inputs := [][]string{
				{"--loss=50", "--dry-run=client"},
				{"--loss=50", "--correlation=100", "--dry-run=client"},
			}
			o := NewNetworkChaosOptions(tf, streams, string(v1alpha1.LossAction))
			cmd := o.NewCobraCommand(Loss, LossShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringVar(&o.Loss, "loss", "", `Indicates the probability of a packet error occurring. Value range: [0, 100].`)
			cmd.Flags().StringVarP(&o.Correlation, "correlation", "c", "0", `Indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100].`)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault network delay", func() {
			inputs := [][]string{
				{"--latency=50s", "--jitter=10s", "--dry-run=client"},
				{"--latency=50s", "--jitter=10s", "--correlation=100", "--dry-run=client"},
				{"--latency=50s", "--correlation=100", "--dry-run=client"},
			}
			o := NewNetworkChaosOptions(tf, streams, string(v1alpha1.DelayAction))
			cmd := o.NewCobraCommand(Delay, DelayShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringVar(&o.Latency, "latency", "", `the length of time to delay.`)
			cmd.Flags().StringVar(&o.Jitter, "jitter", "0ms", `the variation range of the delay time.`)
			cmd.Flags().StringVarP(&o.Correlation, "correlation", "c", "0", `Indicates the probability of a packet error occurring. Value range: [0, 100].`)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault network duplicate", func() {
			inputs := [][]string{
				{"--duplicate=50", "--dry-run=client"},
				{"--duplicate=50", "--correlation=100", "--dry-run=client"},
			}
			o := NewNetworkChaosOptions(tf, streams, string(v1alpha1.DuplicateAction))
			cmd := o.NewCobraCommand(Duplicate, DuplicateShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringVar(&o.Duplicate, "duplicate", "", `the probability of a packet being repeated. Value range: [0, 100].`)
			cmd.Flags().StringVarP(&o.Correlation, "correlation", "c", "0", `Indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100].`)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault network corrupt", func() {
			inputs := [][]string{
				{"--corrupt=50", "--dry-run=client"},
				{"--corrupt=50", "--correlation=100", "--dry-run=client"},
			}
			o := NewNetworkChaosOptions(tf, streams, string(v1alpha1.CorruptAction))
			cmd := o.NewCobraCommand(Corrupt, CorruptShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringVar(&o.Corrupt, "corrupt", "", `Indicates the probability of a packet error occurring. Value range: [0, 100].`)
			cmd.Flags().StringVarP(&o.Correlation, "correlation", "c", "0", `Indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100].`)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault network bandwidth", func() {
			inputs := [][]string{
				{"--rate=10kbps", "--dry-run=client"},
				{"--rate=10kbps", "--limit=1000", "--buffer=100", "--peakrate=10", "--minburst=5", "--dry-run=client"},
			}
			o := NewNetworkChaosOptions(tf, streams, string(v1alpha1.BandwidthAction))
			cmd := o.NewCobraCommand(Bandwidth, BandwidthShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringVar(&o.Rate, "rate", "", `the rate at which the bandwidth is limited. For example : 10 bps/kbps/mbps/gbps.`)
			cmd.Flags().Uint32Var(&o.Limit, "limit", 1, `the number of bytes waiting in the queue.`)
			cmd.Flags().Uint32Var(&o.Buffer, "buffer", 1, `the maximum number of bytes that can be sent instantaneously.`)
			cmd.Flags().Uint64Var(&o.Peakrate, "peakrate", 0, `the maximum consumption rate of the bucket.`)
			cmd.Flags().Uint32Var(&o.Minburst, "minburst", 0, `the size of the peakrate bucket.`)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})
	})
})
