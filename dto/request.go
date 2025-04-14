package dto

import (
	"context"
	"encoding/json"
	"strings"
)

// GeneralOpenAIRequest represents a general request to OpenAI API
type GeneralOpenAIRequest struct {
	Model            string          `json:"model"`
	Messages         []Message       `json:"messages"`
	Stream           bool            `json:"stream,omitempty"`
	Temperature      *float64        `json:"temperature,omitempty"`
	MaxTokens        uint            `json:"max_tokens,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64        `json:"presence_penalty,omitempty"`
	Stop             []string        `json:"stop,omitempty"`
	Input            any             `json:"input,omitempty"`
	ResponseFormat   *ResponseFormat `json:"response_format,omitempty"`
}

// ResponseFormat specifies the format for response
type ResponseFormat struct {
	Type string `json:"type"`
}

// ToolCallRequest represents a tool call in a message
type ToolCallRequest struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function FunctionRequest `json:"function"`
}

// FunctionRequest represents a function request
type FunctionRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// MessageImageUrl represents an image URL in a message
type MessageImageUrl struct {
	Url    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// MessageInputAudio represents audio input in a message
type MessageInputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

// MediaContent represents media content in a message
type MediaContent struct {
	Type       string             `json:"type"`
	Text       string             `json:"text,omitempty"`
	ImageUrl   *MessageImageUrl   `json:"image_url,omitempty"`
	InputAudio *MessageInputAudio `json:"input_audio,omitempty"`
}

func (r GeneralOpenAIRequest) ParseInput() []string {
	if r.Input == nil {
		return nil
	}
	var input []string
	switch r.Input.(type) {
	case string:
		input = []string{r.Input.(string)}
	case []any:
		input = make([]string, 0, len(r.Input.([]any)))
		for _, item := range r.Input.([]any) {
			if str, ok := item.(string); ok {
				input = append(input, str)
			}
		}
	}
	return input
}

type Message struct {
	Role                string          `json:"role"`
	Content             json.RawMessage `json:"content"`
	Name                *string         `json:"name,omitempty"`
	Prefix              *bool           `json:"prefix,omitempty"`
	ReasoningContent    string          `json:"reasoning_content,omitempty"`
	Reasoning           string          `json:"reasoning,omitempty"`
	ToolCalls           json.RawMessage `json:"tool_calls,omitempty"`
	ToolCallId          string          `json:"tool_call_id,omitempty"`
	parsedContent       []MediaContent
	parsedStringContent *string
}

const (
	ContentTypeText       = "text"
	ContentTypeImageURL   = "image_url"
	ContentTypeInputAudio = "input_audio"
)

func (m *Message) GetPrefix() bool {
	if m.Prefix == nil {
		return false
	}
	return *m.Prefix
}

func (m *Message) SetPrefix(prefix bool) {
	m.Prefix = &prefix
}

func (m *Message) ParseToolCalls() ([]ToolCallRequest, error) {
	if m.ToolCalls == nil || len(m.ToolCalls) == 0 {
		return nil, nil
	}

	var toolCalls []ToolCallRequest
	if err := json.Unmarshal(m.ToolCalls, &toolCalls); err != nil {
		return nil, err
	}
	return toolCalls, nil
}

func (m *Message) SetToolCalls(toolCalls any) error {
	toolCallsJson, err := json.Marshal(toolCalls)
	if err != nil {
		return err
	}
	m.ToolCalls = toolCallsJson
	return nil
}

func (m *Message) StringContent() string {
	if m.parsedStringContent != nil {
		return *m.parsedStringContent
	}

	// Try to unmarshal content as a string
	var stringContent string
	if err := json.Unmarshal(m.Content, &stringContent); err == nil {
		m.parsedStringContent = &stringContent
		return stringContent
	}

	// If that fails, try to parse content as media content array
	contentStr := new(strings.Builder)
	arrayContent := m.ParseContent()
	for _, content := range arrayContent {
		if content.Type == ContentTypeText {
			contentStr.WriteString(content.Text)
		}
	}
	stringContent = contentStr.String()
	m.parsedStringContent = &stringContent

	return stringContent
}

func (m *Message) SetStringContent(content string) {
	jsonContent, _ := json.Marshal(content)
	m.Content = jsonContent
	m.parsedStringContent = &content
	m.parsedContent = nil
}

func (m *Message) SetMediaContent(content []MediaContent) {
	jsonContent, _ := json.Marshal(content)
	m.Content = jsonContent
	m.parsedContent = nil
	m.parsedStringContent = nil
}

func (m *Message) IsStringContent() bool {
	if m.parsedStringContent != nil {
		return true
	}

	var stringContent string
	if err := json.Unmarshal(m.Content, &stringContent); err == nil {
		m.parsedStringContent = &stringContent
		return true
	}
	return false
}

func (m *Message) ParseContent() []MediaContent {
	if m.parsedContent != nil {
		return m.parsedContent
	}

	var contentList []MediaContent

	// Try to unmarshal as string first
	var stringContent string
	if err := json.Unmarshal(m.Content, &stringContent); err == nil {
		contentList = []MediaContent{{
			Type: ContentTypeText,
			Text: stringContent,
		}}
		m.parsedContent = contentList
		return contentList
	}

	// Try to unmarshal as array of content items
	var arrayContent []map[string]interface{}
	if err := json.Unmarshal(m.Content, &arrayContent); err == nil {
		for _, contentItem := range arrayContent {
			contentType, ok := contentItem["type"].(string)
			if !ok {
				continue
			}

			switch contentType {
			case ContentTypeText:
				if text, ok := contentItem["text"].(string); ok {
					contentList = append(contentList, MediaContent{
						Type: ContentTypeText,
						Text: text,
					})
				}

			case ContentTypeImageURL:
				imageUrl := contentItem["image_url"]
				temp := &MessageImageUrl{
					Detail: "high",
				}
				switch v := imageUrl.(type) {
				case string:
					temp.Url = v
				case map[string]interface{}:
					url, ok1 := v["url"].(string)
					detail, ok2 := v["detail"].(string)
					if ok2 {
						temp.Detail = detail
					}
					if ok1 {
						temp.Url = url
					}
				}
				contentList = append(contentList, MediaContent{
					Type:     ContentTypeImageURL,
					ImageUrl: temp,
				})

			case ContentTypeInputAudio:
				if audioData, ok := contentItem["input_audio"].(map[string]interface{}); ok {
					data, ok1 := audioData["data"].(string)
					format, ok2 := audioData["format"].(string)
					if ok1 && ok2 {
						temp := &MessageInputAudio{
							Data:   data,
							Format: format,
						}
						contentList = append(contentList, MediaContent{
							Type:       ContentTypeInputAudio,
							InputAudio: temp,
						})
					}
				}
			}
		}
	}

	if len(contentList) > 0 {
		m.parsedContent = contentList
	}
	return contentList
}

type Adapter interface {
	Init(baseURL, apiKey string) error
	ChatCompletion(ctx context.Context, request *GeneralOpenAIRequest) (*OpenAITextResponse, error)
	ChatCompletionStream(ctx context.Context, request *GeneralOpenAIRequest) (chan *ChatCompletionsStreamResponse, error)
}
