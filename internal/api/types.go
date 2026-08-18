package api

import (
	"encoding/json"
	"strconv"
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

type SendPhoneCodeRequest struct {
	CountryCode string `json:"country_code"`
	PhoneNumber string `json:"phone_number"`
	Scene       string `json:"scene"`
}

type PhoneOTPLoginRequest struct {
	CountryCode      string `json:"country_code"`
	PhoneNumber      string `json:"phone_number"`
	VerificationCode string `json:"verification_code"`
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

// StreamMode is the PAVO generation mode selected for a streamed turn.
type StreamMode string

const (
	StreamModeShortDrama    StreamMode = "short_drama"
	StreamModeGenerateImage StreamMode = "generate_image"
	StreamModeGenerateVideo StreamMode = "generate_video"
	StreamModeFramesToVideo StreamMode = "frames_to_video"
)

// ModeCode is a mode supported by the model catalogue endpoint.
type ModeCode string

const (
	ModeCodeShortDrama    ModeCode = "short_drama"
	ModeCodeGenerateImage ModeCode = "generate_image"
	ModeCodeGenerateVideo ModeCode = "generate_video"
)

// ModelTag is localized display metadata returned by Pixa.
type ModelTag struct {
	Code     string `json:"code"`
	I18nCode string `json:"i18n_code"`
	Label    string `json:"label"`
}

// SupportedModel describes one model currently configured for a Pixa mode.
// Modes is populated for generate_video and Type for short_drama.
type SupportedModel struct {
	Code              string     `json:"code"`
	Name              string     `json:"name"`
	ModelIntro        string     `json:"model_intro"`
	IconURL           string     `json:"icon_url"`
	IsOnline          bool       `json:"is_online"`
	ProductName       string     `json:"product_name,omitempty"`
	SubscriptionLevel float64    `json:"subscription_level"`
	Type              string     `json:"type,omitempty"`
	Tags              []ModelTag `json:"tags"`
	Modes             []string   `json:"modes,omitempty"`
}

// VisualCategory selects one of the current user's generated-media libraries.
type VisualCategory string

const (
	VisualCategoryImages          VisualCategory = "images"
	VisualCategoryVideos          VisualCategory = "videos"
	VisualCategoryShortDramaFinal VisualCategory = "short_drama_final"
)

type VisualPagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

type VisualsData struct {
	Pagination VisualPagination `json:"pagination"`
	Groups     []VisualGroup    `json:"groups"`
}

type VisualGroup struct {
	Date string       `json:"date"`
	List []VisualItem `json:"list"`
}

// VisualItem keeps category-specific metadata as raw JSON so new image,
// video, and short-drama fields pass through without a CLI update.
type VisualItem struct {
	VisualID      ConversationID  `json:"visual_id"`
	Source        string          `json:"source"`
	ResourceID    ConversationID  `json:"resource_id"`
	Type          string          `json:"type"`
	URL           string          `json:"url"`
	ThumbnailURL  string          `json:"thumbnail_url"`
	CreatedAt     string          `json:"created_at"`
	Metadata      json.RawMessage `json:"metadata"`
	LocalPath     string          `json:"local_path,omitempty"`
	DownloadError string          `json:"download_error,omitempty"`
}

// StreamOptions configures one new streamed turn. Resume calls never reuse
// these fields because they reconnect to an already submitted turn.
type StreamOptions struct {
	Mode         StreamMode
	Files        []ChatAttachment
	ExtraContext *StreamExtraContext
	Creative     *CreativeGenerationOptions
}

// CreativeGenerationOptions contains the shared Pixa fields used by image,
// omni-video, and frames-to-video generation. Flexible fields are encoded as
// raw JSON so the CLI can preserve the API's explicit "auto" value.
type CreativeGenerationOptions struct {
	Model              string
	Ratio              string
	Resolution         string
	Duration           json.RawMessage
	Count              json.RawMessage
	Sound              json.RawMessage
	Images             []MediaReference
	Videos             []MediaReference
	Audios             []MediaReference
	CreativePromptJSON string
}

// MediaReference is the object shape required by Pixa for top-level image,
// video, and audio references.
type MediaReference struct {
	URL string `json:"url"`
}

// StreamExtraContext contains optional mode-specific configuration accepted
// by the PAVO chat stream endpoint.
type StreamExtraContext struct {
	ShortDramaParams *ShortDramaModelParams `json:"agent_params,omitempty"`
}

// ShortDramaModelParams selects the image and video models used by a
// short-drama turn. Both model codes are required by the backend contract.
type ShortDramaModelParams struct {
	ImageModelCode string `json:"image_model_code,omitempty"`
	VideoModelCode string `json:"video_model_code,omitempty"`
}

type StreamRequest struct {
	ConversationID     ConversationID      `json:"conversation_id"`
	Prompt             string              `json:"prompt"`
	Mode               string              `json:"mode"`
	Model              string              `json:"model,omitempty"`
	Ratio              string              `json:"ratio,omitempty"`
	Resolution         string              `json:"resolution,omitempty"`
	Duration           json.RawMessage     `json:"duration,omitempty"`
	Count              json.RawMessage     `json:"count,omitempty"`
	Sound              json.RawMessage     `json:"sound,omitempty"`
	Images             []MediaReference    `json:"images,omitempty"`
	Videos             []MediaReference    `json:"videos,omitempty"`
	Audios             []MediaReference    `json:"audios,omitempty"`
	CreativePromptJSON string              `json:"creative_prompt_json,omitempty"`
	Files              []ChatAttachment    `json:"files,omitempty"`
	ExtraContext       *StreamExtraContext `json:"extra_context,omitempty"`
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
		var results []GenerationResult
		seen := make(map[string]struct{})
		for _, block := range blocks {
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
			for index, result := range payload.Results {
				key := generationResultKey(result, block.Type, index)
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				results = append(results, result)
			}
		}
		if len(results) > 0 {
			return results
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

// GeneratedAsset retains the identity of one image, video, or other media
// item emitted by a streamed generation event. Result is duplicated in
// StreamOutput.Results for compatibility with existing callers.
type GeneratedAsset struct {
	EventID   string           `json:"event_id,omitempty"`
	EventType string           `json:"event_type,omitempty"`
	Group     string           `json:"group,omitempty"`
	ItemID    string           `json:"item_id,omitempty"`
	Kind      string           `json:"kind,omitempty"`
	TaskID    string           `json:"task_id,omitempty"`
	Title     string           `json:"title,omitempty"`
	Result    GenerationResult `json:"result"`
}

// AssistantMessage is the complete text reconstructed from MessageDelta
// events for one server message ID.
type AssistantMessage struct {
	MessageID string `json:"message_id,omitempty"`
	Content   string `json:"content"`
}

type EventData struct {
	Content   string             `json:"content,omitempty"`
	EventID   string             `json:"event_id"`
	Extra     json.RawMessage    `json:"extra,omitempty"`
	Kind      string             `json:"kind,omitempty"`
	MessageID string             `json:"message_id"`
	ModelCode string             `json:"model_code"`
	Message   string             `json:"message,omitempty"`
	Results   []GenerationResult `json:"results,omitempty"`
	TaskID    string             `json:"task_id"`
	Title     string             `json:"title,omitempty"`
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
	ConversationID    string             `json:"conversation_id"`
	TerminalType      string             `json:"terminal_type"`
	EventID           string             `json:"event_id"`
	MessageID         string             `json:"message_id"`
	ModelCode         string             `json:"model_code"`
	TaskID            string             `json:"task_id"`
	TraceID           string             `json:"trace_id"`
	Results           []GenerationResult `json:"results"`
	Assets            []GeneratedAsset   `json:"assets,omitempty"`
	AssistantText     string             `json:"assistant_text,omitempty"`
	AssistantMessages []AssistantMessage `json:"assistant_messages,omitempty"`
	Review            json.RawMessage    `json:"review,omitempty"`
	Artifacts         []json.RawMessage  `json:"artifacts,omitempty"`
}

func generationResultKey(result GenerationResult, eventType string, index int) string {
	if value := strings.TrimSpace(result.URL); value != "" {
		return "url:" + value
	}
	if value := strings.TrimSpace(result.Base64); value != "" {
		return "base64:" + value
	}
	return eventType + ":" + result.Mimetype + ":" + result.Message + ":" + strconv.Itoa(index)
}
