/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package consensusset

type Plan struct {
	Start    *Step
	WalkFunc WalkFunc
}

type Step struct {
	Obj       interface{}
	NextSteps []*Step
}

type WalkFunc func(obj interface{}) (bool, error)

func (p *Plan) WalkOneStep() (bool, error) {
	if len(p.Start.NextSteps) == 0 {
		return true, nil
	}

	shouldStop := false
	for _, step := range p.Start.NextSteps {
		walked, err := p.WalkFunc(step.Obj)
		if err != nil {
			return false, err
		}
		if walked {
			shouldStop = true
		}
	}

	if shouldStop {
		return false, nil
	}

	// generate new plan
	plan := &Plan{}
	plan.Start = &Step{}
	plan.WalkFunc = p.WalkFunc
	plan.Start.NextSteps = make([]*Step, 0)
	for _, step := range p.Start.NextSteps {
		for _, nextStep := range step.NextSteps {
			if !containStep(plan.Start.NextSteps, nextStep) {
				plan.Start.NextSteps = append(plan.Start.NextSteps, nextStep)
			}
		}
	}

	return plan.WalkOneStep()
}

func containStep(steps []*Step, step *Step) bool {
	for _, s := range steps {
		if s == step {
			return true
		}
	}

	return false
}
