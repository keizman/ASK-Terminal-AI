package relay

import (
	"ask_terminal/common"
	"ask_terminal/dto"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type OpenAIAdapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewOpenAIAdapter() *OpenAIAdapter {
	return &OpenAIAdapter{
		client: &http.Client{},
	}
}

func (a *OpenAIAdapter) Init(baseURL, apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("apiKey cannot be empty")
	}

	// Use default URL if baseURL is empty
	if baseURL == "" {
		baseURL = common.DefaultBaseURL
	}

	// Ensure URL ends with a "/" for proper endpoint joining
	a.baseURL = strings.TrimRight(baseURL, "/") + "/"
	a.apiKey = apiKey
	return nil
}

func (a *OpenAIAdapter) ChatCompletion(ctx context.Context, request *dto.GeneralOpenAIRequest) (*dto.OpenAITextResponse, error) {
	endpoint := "chat/completions"
	url := a.baseURL + endpoint

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.client.Do(req)
	// Log the request and response for debugging
	bodyPreview, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyPreview)) // Reassign body for further use

	// Replace the logging line after the client.Do(req) call with:

	log.Printf("Request URL: %s", url)
	log.Printf("Request Headers: %+v", req.Header)
	log.Printf("Request Body: %s", string(jsonData)) // We already have the request body in jsonData
	log.Printf("Response Status: %d", resp.StatusCode)
	log.Printf("Response Body: %s", string(bodyPreview))

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp dto.GeneralErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			// Print full error details
			log.Printf("Full API error response: %s", string(body))
			return nil, fmt.Errorf("API error: %s (Status code: %d) - Error: %+v",
				errResp.ToMessage(),
				resp.StatusCode,
				errResp)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result dto.OpenAITextResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (a *OpenAIAdapter) ChatCompletionStream(ctx context.Context, request *dto.GeneralOpenAIRequest) (chan *dto.ChatCompletionsStreamResponse, error) {
	// Set stream to true for streaming response
	request.Stream = true

	endpoint := "chat/completions"
	url := a.baseURL + endpoint

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Full API error response (stream): %s", string(body))
		var errResp dto.GeneralErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("API error: %s (Status code: %d) - Error: %+v",
				errResp.ToMessage(),
				resp.StatusCode,
				errResp)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	responseChannel := make(chan *dto.ChatCompletionsStreamResponse)

	go func() {
		defer resp.Body.Close()
		defer close(responseChannel)

		reader := bufio.NewReader(resp.Body)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				line, err := reader.ReadBytes('\n')
				if err != nil {
					if err != io.EOF {
						log.Printf("Error reading stream: %v", err)
					}
					return
				}

				line = bytes.TrimSpace(line)
				if len(line) == 0 {
					continue
				}

				if bytes.HasPrefix(line, []byte("data: ")) {
					data := bytes.TrimPrefix(line, []byte("data: "))

					// Check for [DONE] message
					if bytes.Equal(data, []byte("[DONE]")) {
						return
					}

					var streamResponse dto.ChatCompletionsStreamResponse
					if err := json.Unmarshal(data, &streamResponse); err != nil {
						log.Printf("Error parsing stream response: %v", err)
						continue
					}

					responseChannel <- &streamResponse
				}
			}
		}
	}()

	return responseChannel, nil
}

// ProcessQuery implements the AIAdapter interface for simple query processing
func (a *OpenAIAdapter) ProcessQuery(query string) (string, error) {
	ctx := context.Background()

	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o-mini", // Default model
		Messages: []dto.Message{
			{
				Role: "user",
			},
		},
	}

	request.Messages[0].SetStringContent(query)

	response, err := a.ChatCompletion(ctx, request)
	if err != nil {
		return "", err
	}

	if len(response.Choices) > 0 {
		return response.Choices[0].Message.StringContent(), nil
	}

	return "", nil
}
