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

package util

type Plan struct {
	Start    *Step
	WalkFunc WalkFunc
}

type Step struct {
	Obj       interface{}
	NextSteps []*Step
}

type WalkFunc func(obj interface{}) (bool, error)

// WalkOneStep process plan stepping
// @return isCompleted
// @return err
func (p *Plan) WalkOneStep() (bool, error) {
	if p == nil {
		return true, nil
	}

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
