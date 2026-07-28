package api

import (
	"encoding/json"
	"strings"
)

const SuccessCode = "000000"

type UserInfo struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	AvatarURL    string `json:"avatar_url"`
	Email        string `json:"email"`
	CountryCode  string `json:"country_code"`
	PhoneNumber  string `json:"phone_number"`
	AuthProvider string `json:"auth_provider"`
	AppID        string `json:"app_id"`
	IsActive     bool   `json:"is_active"`
	IsNewAccount bool   `json:"is_new_account"`
	CreatedAt    int64  `json:"created_at"`
}

type LoginRequest struct {
	EmailPassword EmailPassword `json:"email_password"`
}

type EmailPassword struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginData struct {
	AccessToken string   `json:"access_token"`
	UserInfo    UserInfo `json:"user_info"`
}

type LoginResult struct {
	AccessToken string
	UserInfo    UserInfo
}

type CreateConversationRequest struct {
	Title    string `json:"title"`
	FolderID string `json:"folder_id"`
	KBStrict bool   `json:"kb_strict"`
}

type ConversationID string

func (id *ConversationID) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*id = ConversationID(text)
		return nil
	}

	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	*id = ConversationID(number.String())
	return nil
}

func (id ConversationID) MarshalJSON() ([]byte, error) {
	value := strings.TrimSpace(string(id))
	if value != "" && (len(value) == 1 || value[0] != '0') {
		numeric := true
		for _, char := range value {
			if char < '0' || char > '9' {
				numeric = false
				break
			}
		}
		if numeric {
			return []byte(value), nil
		}
	}
	return json.Marshal(value)
}

type CreateConversationData struct {
	ConversationID ConversationID `json:"conversation_id"`
}

type PresignedURLRequest struct {
	Purpose     string `json:"purpose"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename"`
}

type PresignedURLData struct {
	UploadURL       string            `json:"upload_url"`
	PublicURL       string            `json:"public_url"`
	Method          string            `json:"method"`
	ExpiresIn       int64             `json:"expires_in"`
	RequiredHeaders map[string]string `json:"required_headers"`
}

// FileUploadResult intentionally excludes the temporary signed upload URL.
type FileUploadResult struct {
	PublicURL   string `json:"public_url"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename"`
}

type StreamRequest struct {
	ConversationID ConversationID   `json:"conversation_id"`
	Prompt         string           `json:"prompt"`
	Mode           string           `json:"mode"`
	Files          []ChatAttachment `json:"files,omitempty"`
}

type ChatAttachment struct {
	MimeType string `json:"mime_type"`
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

// ResumeStreamRequest reconnects to an in-progress stream. FromSeq is the
// highest sequence number already processed by the caller; the service only
// replays events after that point.
type ResumeStreamRequest struct {
	ConversationID ConversationID `json:"conversation_id"`
	FromSeq        int64          `json:"from_seq,omitempty"`
}

// ConversationStatus is the lightweight real-time status of a conversation.
type ConversationStatus struct {
	ConversationID ConversationID `json:"conversation_id"`
	IsRunning      bool           `json:"is_running"`
	RequestID      string         `json:"request_id"`
}

// ConversationHistory is the durable fallback once the short-lived stream
// replay buffer has expired.
type ConversationHistory struct {
	IsRunning bool               `json:"is_running"`
	Turns     []ConversationTurn `json:"turns"`
}

type ConversationTurn struct {
	RequestID string         `json:"request_id"`
	Assistant []ContentBlock `json:"assistant"`
}

type ContentBlock struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// LatestGenerationResults returns the newest persisted generation results in
// the conversation. The service stores them as GenerationSuccess or
// GenerationArtifact assistant blocks.
func (h *ConversationHistory) LatestGenerationResults() []GenerationResult {
	if h == nil {
		return nil
	}
	for turnIndex := len(h.Turns) - 1; turnIndex >= 0; turnIndex-- {
		blocks := h.Turns[turnIndex].Assistant
		for blockIndex := len(blocks) - 1; blockIndex >= 0; blockIndex-- {
			block := blocks[blockIndex]
			if block.Type != "GenerationSuccess" && block.Type != "GenerationArtifact" {
				continue
			}
			var payload struct {
				Results []GenerationResult `json:"results"`
			}
			if err := json.Unmarshal(block.Data, &payload); err != nil || len(payload.Results) == 0 {
				continue
			}
			if block.Type == "GenerationArtifact" {
				for index := range payload.Results {
					payload.Results[index].Success = true
				}
			}
			return payload.Results
		}
	}
	return nil
}

type GenerationResult struct {
	Base64       string `json:"base64,omitempty"`
	Height       int    `json:"height"`
	Message      string `json:"message"`
	LocalPath    string `json:"local_path,omitempty"`
	Mimetype     string `json:"mimetype"`
	Ratio        string `json:"ratio"`
	Success      bool   `json:"success"`
	ThumbnailURL string `json:"thumbnail_url"`
	URL          string `json:"url"`
	Width        int    `json:"width"`
}

type EventData struct {
	EventID   string             `json:"event_id"`
	MessageID string             `json:"message_id"`
	ModelCode string             `json:"model_code"`
	Message   string             `json:"message,omitempty"`
	Results   []GenerationResult `json:"results,omitempty"`
	TaskID    string             `json:"task_id"`
	TraceID   string             `json:"trace_id"`
}

type StreamEvent struct {
	AppStandaloneCard bool            `json:"app_standalone_card"`
	Data              EventData       `json:"data"`
	EventID           string          `json:"event_id"`
	MessageID         string          `json:"message_id"`
	ModelCode         string          `json:"model_code"`
	Seq               int64           `json:"seq"`
	TaskID            string          `json:"task_id"`
	Timestamp         int64           `json:"ts"`
	TraceID           string          `json:"trace_id"`
	Type              string          `json:"type"`
	Raw               json.RawMessage `json:"-"`
}

type StreamOutput struct {
	ConversationID string             `json:"conversation_id"`
	TerminalType   string             `json:"terminal_type"`
	EventID        string             `json:"event_id"`
	MessageID      string             `json:"message_id"`
	ModelCode      string             `json:"model_code"`
	TaskID         string             `json:"task_id"`
	TraceID        string             `json:"trace_id"`
	Results        []GenerationResult `json:"results"`
	Artifacts      []json.RawMessage  `json:"artifacts,omitempty"`
}
