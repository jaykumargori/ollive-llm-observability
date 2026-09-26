// Package evals contains deterministic evaluation math; the judge can be swapped
// without changing the golden-set format or CI gate.
package evals

import (
	"math"
	"strings"
)

type GoldenCase struct {
	ID                string   `json:"id"`
	Prompt            string   `json:"prompt"`
	CandidateResponse string   `json:"candidate_response"`
	RequiredTerms     []string `json:"required_terms"`
	HumanScore        float64  `json:"human_score"`
}
type Result struct {
	ID         string  `json:"id"`
	JudgeScore float64 `json:"judge_score"`
	HumanScore float64 `json:"human_score"`
}
type Report struct {
	Results        []Result `json:"results"`
	MeanScore      float64  `json:"mean_score"`
	CalibrationMAE float64  `json:"calibration_mae"`
}
type Judge interface{ Score(GoldenCase) float64 }

// KeywordJudge is the offline baseline. Production can replace Judge with an
// LLM judge while the same human calibration and gate are retained.
type KeywordJudge struct{}

func (KeywordJudge) Score(c GoldenCase) float64 {
	if len(c.RequiredTerms) == 0 {
		return 1
	}
	answer := strings.ToLower(c.CandidateResponse)
	found := 0
	for _, term := range c.RequiredTerms {
		if strings.Contains(answer, strings.ToLower(term)) {
			found++
		}
	}
	return float64(found) / float64(len(c.RequiredTerms))
}
func Run(cases []GoldenCase, judge Judge) Report {
	r := Report{}
	for _, c := range cases {
		score := judge.Score(c)
		r.Results = append(r.Results, Result{c.ID, score, c.HumanScore})
		r.MeanScore += score
		r.CalibrationMAE += math.Abs(score - c.HumanScore)
	}
	if len(cases) > 0 {
		r.MeanScore /= float64(len(cases))
		r.CalibrationMAE /= float64(len(cases))
	}
	return r
}
