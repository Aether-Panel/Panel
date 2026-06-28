package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/gin-gonic/gin"
)

type AIAnalyzeReq struct {
	Logs []string `json:"logs" binding:"required"`
}

func RegisterAIRoutes(rg *gin.RouterGroup) {
	rg.POST("/analyze", analyzeServerLogs)
}

type GeminiContent struct {
	Parts []struct {
		Text string `json:"text"`
	} `json:"parts"`
}

type GeminiMessage struct {
	Contents          []GeminiContent        `json:"contents"`
	SystemInstruction *GeminiContent         `json:"systemInstruction,omitempty"`
	GenerationConfig  map[string]interface{} `json:"generationConfig,omitempty"`
}

func analyzeServerLogs(c *gin.Context) {
	apiKey := config.GeminiApiKey.Value()
	if apiKey == "" {
		c.JSON(400, gin.H{"error": "Gemini API Key is not configured. Please add it in Settings."})
		return
	}

	var req AIAnalyzeReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid payload"})
		return
	}

	if len(req.Logs) == 0 {
		c.JSON(400, gin.H{"error": "No logs provided for analysis"})
		return
	}

	logsText := ""
	for _, l := range req.Logs {
		logsText += l + "\n"
	}

	prompt := fmt.Sprintf("Analyze the following server console error logs and provide a troubleshooting summary. Give root causes and practical suggestions to fix the issues.\n\nLogs:\n%s", logsText)

	payload := GeminiMessage{
		GenerationConfig: map[string]interface{}{
			"response_mime_type": "application/json",
			"response_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"summary": map[string]interface{}{"type": "string"},
					"suggestions": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"rootCauses": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
				},
				"required": []string{"summary", "suggestions", "rootCauses"},
			},
		},
		Contents: []GeminiContent{
			{
				Parts: []struct {
					Text string `json:"text"`
				}{{Text: prompt}},
			},
		},
	}

	payloadBytes, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", apiKey)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to connect to AI API"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		c.JSON(500, gin.H{"error": fmt.Sprintf("AI API returned status %d", resp.StatusCode)})
		return
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(500, gin.H{"error": "Failed to parse AI response"})
		return
	}

	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		jsonStr := result.Candidates[0].Content.Parts[0].Text
		var finalResponse map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &finalResponse); err != nil {
			c.JSON(500, gin.H{"error": "AI returned invalid JSON structure"})
			return
		}
		c.JSON(200, finalResponse)
		return
	}

	c.JSON(500, gin.H{"error": "Empty response from AI"})
}
