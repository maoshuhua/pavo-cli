package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

type EventHandler func(*StreamEvent) error

var ErrStreamEndedWithoutTerminal = errors.New("stream 已结束，但没有收到 GenerationSuccess 或 AgentEnd")

// IsRecoverableStreamError identifies errors for which a client can safely
// reconnect through Resume without creating another generation request.
func IsRecoverableStreamError(err error) bool {
	if err == nil || IsAgentStreamBusy(err) || errors.Is(err, ErrStreamEndedWithoutTerminal) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, context.DeadlineExceeded) {
		return err != nil
	}
	var networkErr net.Error
	return errors.As(err, &networkErr)
}

func (c *Client) Stream(ctx context.Context, conversationID, prompt string, handler EventHandler) (*StreamOutput, error) {
	return c.StreamWithOptions(ctx, conversationID, prompt, StreamOptions{
		Mode: StreamModeDesign,
	}, handler)
}

// StreamWithFiles starts a design generation with optional uploaded chat attachments.
func (c *Client) StreamWithFiles(ctx context.Context, conversationID, prompt string, files []ChatAttachment, handler EventHandler) (*StreamOutput, error) {
	return c.StreamWithOptions(ctx, conversationID, prompt, StreamOptions{
		Mode:  StreamModeDesign,
		Files: files,
	}, handler)
}

// StreamWithOptions starts a streamed PAVO turn with an explicit agent mode.
// It is used by the short-drama command to send mode=short_drama together with
// the model codes required by the PAVO short-drama service.
func (c *Client) StreamWithOptions(ctx context.Context, conversationID, prompt string, options StreamOptions, handler EventHandler) (*StreamOutput, error) {
	conversationID = strings.TrimSpace(conversationID)
	prompt = strings.TrimSpace(prompt)
	if conversationID == "" {
		return nil, errors.New("conversation_id 不能为空")
	}
	if prompt == "" {
		return nil, errors.New("prompt 不能为空")
	}
	mode := strings.TrimSpace(string(options.Mode))
	if mode == "" {
		return nil, errors.New("mode 不能为空")
	}
	files, err := normalizeChatAttachments(options.Files)
	if err != nil {
		return nil, err
	}
	extraContext, err := normalizeStreamExtraContext(options.ExtraContext)
	if err != nil {
		return nil, err
	}
	return c.openStream(ctx, c.paths.Stream, conversationID, StreamRequest{
		ConversationID: ConversationID(conversationID),
		Prompt:         prompt,
		Mode:           mode,
		Files:          files,
		ExtraContext:   extraContext,
	}, handler)
}

// Resume replays buffered events and then continues receiving live events for
// a conversation that is already running. It never submits a second job.
func (c *Client) Resume(ctx context.Context, conversationID string, fromSeq int64, handler EventHandler) (*StreamOutput, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, errors.New("conversation_id 不能为空")
	}
	if fromSeq < 0 {
		return nil, errors.New("from_seq 不能为负数")
	}
	if c.paths == nil || strings.TrimSpace(c.paths.ResumeStream) == "" {
		return nil, errors.New("PAVO API 未配置 resume stream 路径")
	}
	return c.openStream(ctx, c.paths.ResumeStream, conversationID, ResumeStreamRequest{
		ConversationID: ConversationID(conversationID),
		FromSeq:        fromSeq,
	}, handler)
}

func (c *Client) openStream(ctx context.Context, path, conversationID string, body any, handler EventHandler) (*StreamOutput, error) {
	requestURL, err := c.resolveURL(path)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("编码 stream 请求失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("构造 stream 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream, application/x-ndjson, application/json")
	setPAVOHeaders(req)
	if err := c.authorize(req); err != nil {
		return nil, err
	}
	resp, err := c.streamHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s 请求失败: %w", requestURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, responseError(resp)
	}

	collector := NewStreamCollector(conversationID)
	consume := func(raw []byte) error {
		event, err := decodeEvent(raw)
		if err != nil {
			return err
		}
		if event == nil {
			return nil
		}
		collector.Add(event)
		if handler != nil {
			if err := handler(event); err != nil {
				return err
			}
		}
		switch event.Type {
		case "GenerationSuccess", "AgentEnd":
			return io.EOF
		case "GenerationFailed", "GenerationFailure", "TaskFailed", "Error":
			message := strings.TrimSpace(event.Data.Message)
			if message == "" {
				message = event.Type
			}
			return fmt.Errorf("PAVO 生成失败: %s", message)
		default:
			return nil
		}
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	reader := bufio.NewReader(resp.Body)
	if strings.Contains(contentType, "text/event-stream") || looksLikeSSE(reader) {
		err = consumeSSE(reader, consume)
	} else {
		err = consumeJSON(reader, consume)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	result := collector.Output()
	if result == nil || result.TerminalType == "" {
		return nil, ErrStreamEndedWithoutTerminal
	}
	return result, nil
}

func normalizeChatAttachments(files []ChatAttachment) ([]ChatAttachment, error) {
	if len(files) == 0 {
		return nil, nil
	}
	normalized := make([]ChatAttachment, len(files))
	for index, file := range files {
		file.MimeType = strings.TrimSpace(file.MimeType)
		file.URL = strings.TrimSpace(file.URL)
		file.Filename = strings.TrimSpace(file.Filename)
		if file.MimeType == "" || file.URL == "" || file.Filename == "" {
			return nil, fmt.Errorf("附件 files[%d] 缺少 mime_type、url 或 filename", index)
		}
		if err := validateHTTPURL(file.URL, fmt.Sprintf("附件 files[%d].url", index)); err != nil {
			return nil, err
		}
		normalized[index] = file
	}
	return normalized, nil
}

func normalizeStreamExtraContext(extraContext *StreamExtraContext) (*StreamExtraContext, error) {
	if extraContext == nil {
		return nil, nil
	}
	if extraContext.AgentParams == nil {
		return nil, errors.New("extra_context.agent_params 不能为空")
	}
	params := &StreamAgentParams{
		ImageModelCode: strings.TrimSpace(extraContext.AgentParams.ImageModelCode),
		VideoModelCode: strings.TrimSpace(extraContext.AgentParams.VideoModelCode),
	}
	if params.ImageModelCode == "" || params.VideoModelCode == "" {
		return nil, errors.New("extra_context.agent_params 需要 image_model_code 和 video_model_code")
	}
	return &StreamExtraContext{AgentParams: params}, nil
}

func looksLikeSSE(reader *bufio.Reader) bool {
	prefix, err := reader.Peek(5)
	return err == nil && string(prefix) == "data:"
}

func consumeJSON(reader io.Reader, consume func([]byte) error) error {
	decoder := json.NewDecoder(reader)
	for {
		var raw json.RawMessage
		err := decoder.Decode(&raw)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("解析 stream JSON 失败: %w", err)
		}
		if err := consume(raw); err != nil {
			return err
		}
	}
}

func consumeSSE(reader *bufio.Reader, consume func([]byte) error) error {
	var data bytes.Buffer
	flush := func() error {
		payload := bytes.TrimSpace(data.Bytes())
		data.Reset()
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			return nil
		}
		return consume(payload)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("读取 SSE stream 失败: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if flushErr := flush(); flushErr != nil {
				return flushErr
			}
		} else if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
		if errors.Is(err, io.EOF) {
			return flush()
		}
	}
}

func decodeEvent(raw []byte) (*StreamEvent, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil
	}
	var envelope struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("解析 stream 事件失败: %w", err)
	}
	if envelope.Code != "" {
		if err := validateEnvelope(envelope.Code, envelope.Message); err != nil {
			return nil, err
		}
		return nil, nil
	}
	var event StreamEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return nil, fmt.Errorf("解析 stream 事件失败: %w", err)
	}
	event.Raw = append(event.Raw[:0], raw...)
	return &event, nil
}
