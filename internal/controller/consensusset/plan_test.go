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

import (
	"testing"
)

func TestWalkOneStep(t *testing.T) {
	plan := &Plan{}
	plan.Start = &Step{}
	plan.Start.NextSteps = make([]*Step, 1)
	plan.WalkFunc = func(obj interface{}) (bool, error) {
		currentPos := obj.(int)
		if currentPos == 2 {
			return true, nil
		}

		return false, nil
	}

	step1 := &Step{}
	step1.Obj = 1
	step1.NextSteps = make([]*Step, 1)
	plan.Start.NextSteps[0] = step1

	step2 := &Step{}
	step2.Obj = 2
	step1.NextSteps[0] = step2

	end, err := plan.WalkOneStep()
	if err != nil {
		t.Errorf("walk error: %v", err)
	}
	if end {
		t.Errorf("walk should not end")
	}

	step2.Obj = 3

	end, err = plan.WalkOneStep()

	if err != nil {
		t.Errorf("walk error: %v", err)
	}
	if !end {
		t.Errorf("walk should end")
	}

}
