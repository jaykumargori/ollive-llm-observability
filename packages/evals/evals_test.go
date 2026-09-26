package evals

import "testing"

func TestRunCalculatesMeanAndCalibration(t *testing.T) {
	r := Run([]GoldenCase{{ID: "a", CandidateResponse: "blue green", RequiredTerms: []string{"blue", "red"}, HumanScore: .5}}, KeywordJudge{})
	if r.MeanScore != .5 || r.CalibrationMAE != 0 {
		t.Fatalf("unexpected report: %#v", r)
	}
}
