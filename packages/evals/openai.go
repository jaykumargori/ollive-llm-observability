package evals

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
)

// OpenAIJudge scores a case through the OpenAI chat-completions API. Run uses
// its score alongside HumanScore to calculate calibration MAE.
type OpenAIJudge struct {
	APIKey, Model string
	Client        *http.Client
}

func NewOpenAIJudge(key, model string) *OpenAIJudge {
	return &OpenAIJudge{APIKey: key, Model: model, Client: http.DefaultClient}
}
func (j *OpenAIJudge) Score(c GoldenCase) float64 {
	if j.APIKey == "" {
		return 0
	}
	prompt := "Return JSON only: {\\\"score\\\":0..1}. Score the response against required concepts. Required: " + join(c.RequiredTerms) + "\\nResponse: " + c.CandidateResponse
	body, _ := json.Marshal(map[string]any{"model": j.Model, "temperature": 0, "response_format": map[string]string{"type": "json_object"}, "messages": []map[string]string{{"role": "system", "content": "You are a strict evaluation judge."}, {"role": "user", "content": prompt}}})
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return 0
	}
	req.Header.Set("Authorization", "Bearer "+j.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := j.Client.Do(req)
	if err != nil || resp.StatusCode >= 300 {
		return 0
	}
	defer resp.Body.Close()
	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.NewDecoder(resp.Body).Decode(&payload) != nil || len(payload.Choices) == 0 {
		return 0
	}
	var score struct {
		Score float64 `json:"score"`
	}
	if json.Unmarshal([]byte(payload.Choices[0].Message.Content), &score) != nil {
		return 0
	}
	if score.Score < 0 {
		return 0
	}
	if score.Score > 1 {
		return 1
	}
	return score.Score
}
func join(items []string) string { b, _ := json.Marshal(items); return string(b) }
func DefaultOpenAIJudge(model string) *OpenAIJudge {
	return NewOpenAIJudge(os.Getenv("OPENAI_API_KEY"), model)
}
