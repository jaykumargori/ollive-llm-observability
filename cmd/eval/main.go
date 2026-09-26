package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"ollive-llm-observability/packages/evals"
	"os"
)

func main() {
	file := flag.String("golden-set", "evals/golden-set.json", "golden set JSON")
	min := flag.Float64("min-score", .80, "minimum mean score")
	maxMAE := flag.Float64("max-calibration-mae", .20, "maximum judge/human MAE")
	judgeName := flag.String("judge", "keyword", "keyword or openai")
	model := flag.String("judge-model", "gpt-4.1-mini", "OpenAI judge model")
	flag.Parse()
	data, err := os.ReadFile(*file)
	if err != nil {
		panic(err)
	}
	var cases []evals.GoldenCase
	if err := json.Unmarshal(data, &cases); err != nil {
		panic(err)
	}
	var judge evals.Judge = evals.KeywordJudge{}
	if *judgeName == "openai" {
		judge = evals.DefaultOpenAIJudge(*model)
	}
	report := evals.Run(cases, judge)
	_ = json.NewEncoder(os.Stdout).Encode(report)
	if report.MeanScore < *min || report.CalibrationMAE > *maxMAE {
		fmt.Fprintln(os.Stderr, "evaluation regression gate failed")
		os.Exit(1)
	}
}
