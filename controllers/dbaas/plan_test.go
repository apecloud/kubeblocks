package dbaas

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

	end, err := plan.walkOneStep()
	if err != nil {
		t.Errorf("walk error: %v", err)
	}
	if end {
		t.Errorf("walk should not end")
	}

	step2.Obj = 3

	end, err = plan.walkOneStep()

	if err != nil {
		t.Errorf("walk error: %v", err)
	}
	if !end {
		t.Errorf("walk should end")
	}

}
