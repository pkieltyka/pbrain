package main

type modelReport struct {
	Name          string      `json:"name"`
	Cutoff        float32     `json:"cutoff"`
	Semantic      modelResult `json:"semantic_gate"`
	Combined      abstention  `json:"combined_abstention_gate"`
	BM25WithModel bool        `json:"bm25_with_model"`
}

type modelResult struct {
	Pass       bool        `json:"pass"`
	Abstention abstention  `json:"abstention"`
	Gain       improvement `json:"gain"`
}

type abstention struct {
	Pass      bool          `json:"pass"`
	Empty     int           `json:"empty"`
	Negatives int           `json:"unanswerable"`
	Failures  []gateFailure `json:"failures,omitempty"`
}

type improvement struct {
	Pass        bool     `json:"pass"`
	StrictFound int      `json:"strict_found"`
	ArmFound    int      `json:"arm_found"`
	Lost        []string `json:"lost_expected_ids_on_questions,omitempty"`
}

func checkAbstention(qs []question, baseline, arm armReport) abstention {
	a := abstention{Empty: arm.Empty, Negatives: arm.Negatives}
	a.Pass = a.Negatives > 0 && a.Empty*100 >= 95*a.Negatives
	for _, q := range qs {
		if q.Kind == unanswered && !arm.results[q.ID].empty {
			a.Failures = append(a.Failures, gateFailure{q.ID, q.Regression, !baseline.results[q.ID].empty})
			a.Pass = a.Pass && !q.Regression
		}
	}
	return a
}

func checkGain(qs []question, baseline, arm armReport) improvement {
	g := improvement{StrictFound: baseline.Found, ArmFound: arm.Found}
	for _, q := range qs {
		if q.Kind == answerable && baseline.results[q.ID].found && !arm.results[q.ID].found {
			g.Lost = append(g.Lost, q.ID)
		}
	}
	g.Pass = g.ArmFound > g.StrictFound && len(g.Lost) == 0
	return g
}

func modelGate(qs []question, baseline, arm armReport) modelResult {
	a := checkAbstention(qs, baseline, arm)
	g := checkGain(qs, baseline, arm)
	return modelResult{Pass: a.Pass && g.Pass, Abstention: a, Gain: g}
}

func partialGate(qs []question, baseline, arm armReport) gate {
	a := checkAbstention(qs, baseline, arm)
	g := checkGain(qs, baseline, arm)
	return gate{
		Pass: a.Pass && g.Pass, Abstention: a.Pass, Gain: g.Pass,
		Empty: a.Empty, Negatives: a.Negatives, StrictFound: g.StrictFound, PartialFound: g.ArmFound,
		Lost: g.Lost, Failures: a.Failures,
	}
}
