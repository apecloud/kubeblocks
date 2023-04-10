package migration

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/apecloud/kubeblocks/internal/cli/types/migrationapi"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("base", func() {

	Context("Basic function validate", func() {

		It("CliStepChangeToStructure", func() {
			resultMap, resultKeyArr := CliStepChangeToStructure()
			Expect(len(resultMap)).Should(Equal(4))
			Expect(len(resultKeyArr)).Should(Equal(4))
		})

		It("TimeFormat", func() {
			time := metav1.NewTime(time.Date(2023, time.Month(4), 4, 15, 31, 0, 0, time.Now().Location()))
			newTime := TimeFormat(&time)
			Expect(newTime).Should(Equal("Apr 04,2023 15:31:00 UTC+0800"))
		})

		It("BuildInitializationStepsOrder", func() {
			task := &v1alpha1.MigrationTask{
				Spec: v1alpha1.MigrationTaskSpec{
					Initialization: v1alpha1.InitializationConfig{
						Steps: []v1alpha1.StepEnum{
							v1alpha1.StepFullLoad,
							v1alpha1.StepStructPreFullLoad,
						},
					},
				},
			}
			template := &v1alpha1.MigrationTemplate{
				Spec: v1alpha1.MigrationTemplateSpec{
					Initialization: v1alpha1.InitializationModel{
						Steps: []v1alpha1.StepModel{
							{Step: v1alpha1.StepStructPreFullLoad},
							{Step: v1alpha1.StepFullLoad},
						},
					},
				},
			}
			arr := BuildInitializationStepsOrder(task, template)
			Expect(len(arr)).Should(Equal(2))
			Expect(arr[0]).Should(Equal(v1alpha1.StepStructPreFullLoad.CliString()))
			Expect(arr[1]).Should(Equal(v1alpha1.StepFullLoad.CliString()))
		})
	})

})
