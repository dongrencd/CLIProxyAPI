package responses

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertOpenAIResponsesRequestToOpenAIChatCompletions_PreservesInstructionRoles(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, arrayContent := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%t/array=%t", stream, arrayContent), func(t *testing.T) {
				wantRoles := []string{"system", "system", "developer", "user", "assistant", "developer", "user", "developer"}
				wantTexts := []string{"base instructions", "session instructions", "mode: plan", "mode: default is only user text", "acknowledged", "mode: default", "continue", "mode: plan"}
				input := make([]any, 0, len(wantRoles)-1)
				for index := 1; index < len(wantRoles); index++ {
					var content any = wantTexts[index]
					if arrayContent {
						contentType := "input_text"
						if wantRoles[index] == "assistant" {
							contentType = "output_text"
						}
						content = []any{map[string]any{"type": contentType, "text": wantTexts[index]}}
					}
					input = append(input, map[string]any{"type": "message", "role": wantRoles[index], "content": content})
				}
				payload, err := json.Marshal(map[string]any{"instructions": wantTexts[0], "input": input})
				if err != nil {
					t.Fatal(err)
				}
				output, err := ConvertOpenAIResponsesRequestToOpenAIChatCompletions("test-model", payload, stream)
				if err != nil {
					t.Fatal(err)
				}
				messages := gjson.GetBytes(output, "messages").Array()
				if len(messages) != len(wantRoles) {
					t.Fatalf("message count = %d, want %d; output=%s", len(messages), len(wantRoles), output)
				}
				for index, message := range messages {
					if got := message.Get("role").String(); got != wantRoles[index] {
						t.Errorf("message %d role = %q, want %q", index, got, wantRoles[index])
					}
					text := message.Get("content")
					if text.IsArray() {
						text = text.Get("0.text")
					}
					if text.String() != wantTexts[index] {
						t.Errorf("message %d text = %q, want %q", index, text.String(), wantTexts[index])
					}
				}
			})
		}
	}
}

func TestConvertOpenAIResponsesRequestToOpenAIChatCompletions_DemotesDeveloperMedia(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantPart    string
		wantPartKey string
	}{
		{name: "image", content: `[{"type":"input_image","image_url":"data:image/png;base64,AA=="}]`, wantPart: "image_url"},
		{name: "video", content: `[{"type":"input_video","video_url":{"url":"https://example.test/video.mp4"}}]`, wantPart: "video_url"},
		{name: "file", content: `[{"type":"input_file","file_id":"file-1"}]`, wantPart: "file"},
		{name: "audio", content: `[{"type":"input_audio","input_audio":{"data":"UklGRg==","format":"wav"}}]`, wantPart: "input_audio"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte(`{"input":[{"type":"message","role":"developer","content":` + tt.content + `}]}`)
			output, err := ConvertOpenAIResponsesRequestToOpenAIChatCompletions("test-model", input, false)
			if err != nil {
				t.Fatal(err)
			}
			message := gjson.GetBytes(output, "messages.0")
			if got := message.Get("role").String(); got != "user" {
				t.Fatalf("role = %q, want user; output=%s", got, output)
			}
			if got := message.Get("content.0.type").String(); got != tt.wantPart {
				t.Fatalf("content type = %q, want %q; output=%s", got, tt.wantPart, output)
			}
		})
	}

	input := []byte(`{"input":[{"type":"message","role":"developer","content":[{"type":"input_text","text":"use the image"},{"type":"input_image","image_url":"data:image/png;base64,AA=="}]}]}`)
	output, err := ConvertOpenAIResponsesRequestToOpenAIChatCompletions("test-model", input, false)
	if err != nil {
		t.Fatal(err)
	}
	message := gjson.GetBytes(output, "messages.0")
	if got := message.Get("role").String(); got != "user" {
		t.Fatalf("mixed media role = %q, want user; output=%s", got, output)
	}
	if got := message.Get("content.#(type= text).text").String(); got != "use the image" {
		t.Fatalf("mixed media text = %q, want %q; output=%s", got, "use the image", output)
	}
}
