package dbaas

type Plan struct {
	Start    *Step
	WalkFunc WalkFunc
}

type Step struct {
	Obj       interface{}
	NextSteps []*Step
}

type WalkFunc func(obj interface{}) (bool, error)

func (p *Plan) walkOneStep() (bool, error) {
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
	plan.WalkFunc = p.WalkFunc
	plan.Start.NextSteps = make([]*Step, 0)
	for _, step := range p.Start.NextSteps {
		for _, nextStep := range step.NextSteps {
			if !containStep(p.Start.NextSteps, nextStep) {
				plan.Start.NextSteps = append(p.Start.NextSteps, nextStep)
			}
		}
	}

	return plan.walkOneStep()
}

func containStep(steps []*Step, step *Step) bool {
	for _, s := range steps {
		if s == step {
			return true
		}
	}

	return false
}
