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
