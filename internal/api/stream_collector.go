package api

import (
	"encoding/json"
	"strings"
)

// StreamCollector collects streamed events into one response. It is safe to
// reuse across a reconnect: events with an already seen positive sequence
// number are ignored so text and generated assets are not duplicated.
type StreamCollector struct {
	conversationID string
	seenSeq        map[int64]struct{}
	assets         []GeneratedAsset
	assetIndex     map[string]int
	artifacts      []json.RawMessage
	messages       map[string]*strings.Builder
	messageOrder   []string
	review         json.RawMessage
	terminal       *StreamEvent
	eventID        string
	messageID      string
	modelCode      string
	taskID         string
	traceID        string
}

func NewStreamCollector(conversationID string) *StreamCollector {
	return &StreamCollector{
		conversationID: strings.TrimSpace(conversationID),
		seenSeq:        make(map[int64]struct{}),
		assetIndex:     make(map[string]int),
		messages:       make(map[string]*strings.Builder),
	}
}

// Add stores one event and returns false when it is a replayed event that was
// already seen by this collector.
func (c *StreamCollector) Add(event *StreamEvent) bool {
	if c == nil || event == nil {
		return false
	}
	if event.Seq > 0 {
		if _, exists := c.seenSeq[event.Seq]; exists {
			return false
		}
		c.seenSeq[event.Seq] = struct{}{}
	}

	c.setMetadata(event)
	if event.Type == "MessageDelta" {
		c.addMessage(event)
	}
	if event.Type == "HumanReview" && len(event.Raw) > 0 {
		c.review = append(c.review[:0], event.Raw...)
	}
	if len(event.Data.Results) > 0 {
		c.addAssets(event)
	}
	if event.Type == "GenerationArtifact" && len(event.Raw) > 0 {
		c.artifacts = append(c.artifacts, append(json.RawMessage(nil), event.Raw...))
	}
	if event.Type == "GenerationSuccess" || event.Type == "AgentEnd" {
		c.terminal = event
	}
	return true
}

func (c *StreamCollector) setMetadata(event *StreamEvent) {
	if value := firstNonEmpty(event.EventID, event.Data.EventID); value != "" {
		c.eventID = value
	}
	if value := firstNonEmpty(event.MessageID, event.Data.MessageID); value != "" {
		c.messageID = value
	}
	if value := firstNonEmpty(event.ModelCode, event.Data.ModelCode); value != "" {
		c.modelCode = value
	}
	if value := firstNonEmpty(event.TaskID, event.Data.TaskID); value != "" {
		c.taskID = value
	}
	if value := firstNonEmpty(event.TraceID, event.Data.TraceID); value != "" {
		c.traceID = value
	}
}

func (c *StreamCollector) addMessage(event *StreamEvent) {
	content := event.Data.Content
	if content == "" {
		// Keep compatibility with older services that used data.message.
		content = event.Data.Message
	}
	if content == "" {
		return
	}
	messageID := firstNonEmpty(event.MessageID, event.Data.MessageID)
	if messageID == "" {
		messageID = "stream"
	}
	builder := c.messages[messageID]
	if builder == nil {
		builder = &strings.Builder{}
		c.messages[messageID] = builder
		c.messageOrder = append(c.messageOrder, messageID)
	}
	builder.WriteString(content)
}

func (c *StreamCollector) addAssets(event *StreamEvent) {
	group, itemID := shortDramaAssetIdentity(event.Data.Extra)
	for index, generated := range event.Data.Results {
		if event.Type == "GenerationArtifact" {
			generated.Success = true
		}
		key := generationResultKey(generated, event.Type, index)
		if _, exists := c.assetIndex[key]; exists {
			continue
		}
		c.assetIndex[key] = len(c.assets)
		c.assets = append(c.assets, GeneratedAsset{
			EventID:   firstNonEmpty(event.EventID, event.Data.EventID),
			EventType: event.Type,
			Group:     group,
			ItemID:    itemID,
			Kind:      event.Data.Kind,
			TaskID:    firstNonEmpty(event.TaskID, event.Data.TaskID),
			Title:     event.Data.Title,
			Result:    generated,
		})
	}
}

func shortDramaAssetIdentity(raw json.RawMessage) (group, itemID string) {
	if len(raw) == 0 {
		return "", ""
	}
	var extra struct {
		ShortDramaItemID        string `json:"short_drama_item_id"`
		ShortDramaParallelGroup string `json:"short_drama_parallel_group"`
	}
	if json.Unmarshal(raw, &extra) != nil {
		return "", ""
	}
	return strings.TrimSpace(extra.ShortDramaParallelGroup), strings.TrimSpace(extra.ShortDramaItemID)
}

func (c *StreamCollector) Output() *StreamOutput {
	if c == nil {
		return nil
	}
	assets := append([]GeneratedAsset(nil), c.assets...)
	results := make([]GenerationResult, len(assets))
	for index := range assets {
		results[index] = assets[index].Result
	}
	messages := make([]AssistantMessage, 0, len(c.messageOrder))
	for _, messageID := range c.messageOrder {
		messages = append(messages, AssistantMessage{
			MessageID: messageID,
			Content:   c.messages[messageID].String(),
		})
	}
	assistantText := ""
	for _, message := range messages {
		if message.MessageID == c.messageID {
			assistantText = message.Content
			break
		}
	}
	if assistantText == "" && len(messages) > 0 {
		assistantText = messages[len(messages)-1].Content
	}
	terminalType := ""
	if c.terminal != nil {
		terminalType = c.terminal.Type
	}
	return &StreamOutput{
		ConversationID:    c.conversationID,
		TerminalType:      terminalType,
		EventID:           c.eventID,
		MessageID:         c.messageID,
		ModelCode:         c.modelCode,
		TaskID:            c.taskID,
		TraceID:           c.traceID,
		Results:           results,
		Assets:            assets,
		AssistantText:     assistantText,
		AssistantMessages: messages,
		Review:            append(json.RawMessage(nil), c.review...),
		Artifacts:         append([]json.RawMessage(nil), c.artifacts...),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
