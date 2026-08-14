package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/spf13/cobra"
)

const (
	defaultImageModel = "agnes-image"
	defaultVideoModel = "agnes-video-new"
)

type creativeCommandOptions struct {
	conversationID string
	prompt         string
	model          string
	ratio          string
	resolution     string
	count          string
	duration       string
	sound          string
	videoMode      string
	images         []string
	videos         []string
	audios         []string
	downloadDir    string
	raw            bool
	liveAssets     bool
}

func newGenerateCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate or edit an image or video through the Pixa creative stream",
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.AddCommand(newCreativeGenerateCommand(stdout, stderr, deps, "image"))
	cmd.AddCommand(newCreativeGenerateCommand(stdout, stderr, deps, "video"))
	return cmd
}

func newCreativeGenerateCommand(stdout, stderr io.Writer, deps *dependencies, kind string) *cobra.Command {
	options := creativeCommandOptions{
		ratio:      "auto",
		resolution: "auto",
		count:      "1",
	}
	mode := api.StreamModeGenerateImage
	modeCode := api.ModeCodeGenerateImage
	if kind == "video" {
		mode = api.StreamModeGenerateVideo
		modeCode = api.ModeCodeGenerateVideo
		options.duration = "auto"
		options.sound = "auto"
		options.videoMode = "auto"
	}
	cmd := &cobra.Command{
		Use:   kind,
		Short: "Generate or edit a PAVO " + kind,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCreativeGenerateCommand(cmd, stdout, stderr, deps, kind, mode, modeCode, &options)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	flags := cmd.Flags()
	flags.StringVar(&options.conversationID, "conversation-id", "", "existing conversation ID; omit to create a new conversation")
	flags.StringVar(&options.prompt, "prompt", "", "generation or edit instruction")
	if kind == "image" {
		flags.StringVar(&options.model, "model", defaultImageModel, "image model code from pavo models --mode generate_image")
	} else {
		flags.StringVar(&options.model, "model", defaultVideoModel, "video model code from pavo models --mode generate_video")
	}
	flags.StringVar(&options.ratio, "ratio", "auto", "output ratio: auto, 1:1, 4:3, 3:4, 16:9, 9:16, 3:2, 2:3, 21:9, 4:5, or 5:4")
	flags.StringVar(&options.resolution, "resolution", "auto", "output resolution: auto, SD, HD, or UHD")
	flags.StringVar(&options.count, "count", "1", "result count: auto or an integer from 1 to 15; model limits may be lower")
	flags.StringArrayVar(&options.images, "image", nil, "local image path or HTTP(S) URL to use as a reference; repeat for multiple images")
	if kind == "video" {
		flags.StringVar(&options.duration, "duration", "auto", "video duration in seconds: auto or an integer from 2 to 15")
		flags.StringVar(&options.sound, "sound", "auto", "video sound: auto, true, or false")
		flags.StringVar(&options.videoMode, "video-mode", "auto", "video input mode: auto, omni_to_video, or frames_to_video")
		flags.StringArrayVar(&options.videos, "video", nil, "local video path or HTTP(S) URL to use as a reference; repeat for multiple videos")
		flags.StringArrayVar(&options.audios, "audio", nil, "local audio path or HTTP(S) URL to use as a reference; repeat for multiple audio files")
	}
	flags.StringVar(&options.downloadDir, "download-dir", "", "directory to save successful generated results")
	flags.BoolVar(&options.raw, "raw", false, "write every raw stream event to stderr")
	flags.BoolVar(&options.liveAssets, "live-assets", false, "write each completed image or video as asset_ready JSONL to stdout")
	return cmd
}

func runCreativeGenerateCommand(
	cmd *cobra.Command,
	stdout io.Writer,
	stderr io.Writer,
	deps *dependencies,
	kind string,
	mode api.StreamMode,
	modeCode api.ModeCode,
	options *creativeCommandOptions,
) error {
	options.conversationID = strings.TrimSpace(options.conversationID)
	options.prompt = strings.TrimSpace(options.prompt)
	options.model = strings.TrimSpace(options.model)
	if options.prompt == "" {
		return errors.New("缺少必填参数 --prompt")
	}
	if options.model == "" {
		return errors.New("缺少必填参数 --model")
	}
	model, err := requireSupportedOnlineModel(cmd.Context(), deps, modeCode, options.model)
	if err != nil {
		return err
	}
	count, err := parseAutoInteger("--count", options.count, 1, 15)
	if err != nil {
		return err
	}
	var duration, sound json.RawMessage
	if kind == "video" {
		mode, err = selectVideoExecutionMode(*model, options.videoMode, len(options.images), len(options.videos), len(options.audios))
		if err != nil {
			return err
		}
		duration, err = parseAutoInteger("--duration", options.duration, 2, 15)
		if err != nil {
			return err
		}
		sound, err = parseAutoBoolean("--sound", options.sound)
		if err != nil {
			return err
		}
	}
	if err := validateKnownAgnesLimits(kind, model.Code, options.ratio, options.resolution, options.count, options.duration); err != nil {
		return err
	}
	if len(options.images) > 5 {
		return errors.New("--image 最多允许 5 项")
	}
	images, err := resolveMediaReferences(cmd.Context(), deps, "image", options.images)
	if err != nil {
		return err
	}
	videos, err := resolveMediaReferences(cmd.Context(), deps, "video", options.videos)
	if err != nil {
		return err
	}
	audios, err := resolveMediaReferences(cmd.Context(), deps, "audio", options.audios)
	if err != nil {
		return err
	}
	creativePromptJSON, err := buildCreativePromptJSON(options.prompt)
	if err != nil {
		return err
	}
	if mode == api.StreamModeFramesToVideo {
		creativePromptJSON = ""
	}
	streamOptions := api.StreamOptions{
		Mode: mode,
		Creative: &api.CreativeGenerationOptions{
			Model:              model.Code,
			Ratio:              options.ratio,
			Resolution:         options.resolution,
			Duration:           duration,
			Count:              count,
			Sound:              sound,
			Images:             images,
			Videos:             videos,
			Audios:             audios,
			CreativePromptJSON: creativePromptJSON,
		},
	}
	conversationID := options.conversationID
	created := false
	if conversationID == "" {
		conversationID, err = deps.api.CreateConversation(cmd.Context(), options.prompt)
		if err != nil {
			return err
		}
		created = true
	}
	result, err := runStreamWithRecovery(cmd.Context(), stdout, stderr, deps, conversationID, options.prompt, streamOptions, 0, streamRunOptions{
		raw:         options.raw,
		liveAssets:  options.liveAssets,
		downloadDir: options.downloadDir,
	}, true)
	if err != nil {
		if created {
			return fmt.Errorf("生成会话 %q 的首轮提交失败；可用 pavo resume --conversation-id %q 恢复：%w", conversationID, conversationID, err)
		}
		return err
	}
	if err := downloadStreamResults(cmd.Context(), deps, result, options.downloadDir); err != nil {
		return err
	}
	return writeStreamResult(stdout, result, options.liveAssets)
}

func requireSupportedOnlineModel(ctx context.Context, deps *dependencies, mode api.ModeCode, code string) (*api.SupportedModel, error) {
	models, err := deps.api.ListModeSupportModels(ctx, mode)
	if err != nil {
		return nil, fmt.Errorf("查询 %s 支持模型失败: %w", mode, err)
	}
	for index := range models {
		if models[index].Code != code {
			continue
		}
		if !models[index].IsOnline {
			return nil, fmt.Errorf("模型 %q 当前未上线；请运行 pavo models --mode %s --online-only", code, mode)
		}
		return &models[index], nil
	}
	return nil, fmt.Errorf("模式 %s 当前不支持模型 %q；请运行 pavo models --mode %s --online-only", mode, code, mode)
}

func selectVideoExecutionMode(model api.SupportedModel, requested string, imageCount, videoCount, audioCount int) (api.StreamMode, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested == "" {
		requested = "auto"
	}
	hasOmni := stringInSlice("omni_to_video", model.Modes)
	hasFrames := stringInSlice("frames_to_video", model.Modes)
	if requested != "auto" && requested != "omni_to_video" && requested != "frames_to_video" {
		return "", errors.New("--video-mode 必须是 auto、omni_to_video 或 frames_to_video")
	}
	if requested == "auto" {
		switch {
		case hasFrames && imageCount <= 2 && videoCount == 0 && audioCount == 0:
			requested = "frames_to_video"
		case hasOmni:
			requested = "omni_to_video"
		case hasFrames:
			requested = "frames_to_video"
		default:
			return "", fmt.Errorf("模型 %q 的目录信息缺少可用视频模式", model.Code)
		}
	}
	if requested == "omni_to_video" {
		if !hasOmni {
			return "", fmt.Errorf("模型 %q 不支持 omni_to_video；请运行 pavo models --mode generate_video --online-only 查看 modes", model.Code)
		}
		return api.StreamModeGenerateVideo, nil
	}
	if !hasFrames {
		return "", fmt.Errorf("模型 %q 不支持 frames_to_video；请运行 pavo models --mode generate_video --online-only 查看 modes", model.Code)
	}
	if imageCount > 2 {
		return "", fmt.Errorf("模型 %q 当前使用 frames_to_video，文生视频不传图片，首尾帧生视频传 1 张首帧图或 2 张首尾帧图", model.Code)
	}
	if videoCount > 0 || audioCount > 0 {
		return "", errors.New("frames_to_video 不接受 --video 或 --audio")
	}
	return api.StreamModeFramesToVideo, nil
}

func resolveMediaReferences(ctx context.Context, deps *dependencies, kind string, values []string) ([]api.MediaReference, error) {
	references := make([]api.MediaReference, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, fmt.Errorf("--%s 不能为空", kind)
		}
		if strings.HasPrefix(strings.ToLower(value), "http://") || strings.HasPrefix(strings.ToLower(value), "https://") {
			parsed, err := url.ParseRequestURI(value)
			if err != nil || parsed.Host == "" {
				return nil, fmt.Errorf("--%s 包含无效 HTTP URL", kind)
			}
			references = append(references, api.MediaReference{URL: value})
			continue
		}
		uploaded, err := deps.api.UploadFile(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("上传 %s 素材 %q 失败: %w", kind, value, err)
		}
		if !strings.HasPrefix(strings.ToLower(uploaded.ContentType), kind+"/") {
			return nil, fmt.Errorf("%q 的媒体类型是 %s，不是 %s", value, uploaded.ContentType, kind)
		}
		references = append(references, api.MediaReference{URL: uploaded.PublicURL})
	}
	return references, nil
}

func buildCreativePromptJSON(prompt string) (string, error) {
	blocks := []struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}{{Type: "text", Content: prompt}}
	payload, err := json.Marshal(blocks)
	if err != nil {
		return "", fmt.Errorf("编码 creative_prompt_json 失败: %w", err)
	}
	return string(payload), nil
}

func parseAutoInteger(name, raw string, min, max int) (json.RawMessage, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "auto" {
		return json.RawMessage(`"auto"`), nil
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < min || number > max {
		return nil, fmt.Errorf("%s 必须是 auto 或 %d 到 %d 的整数", name, min, max)
	}
	return json.RawMessage(strconv.Itoa(number)), nil
}

func parseAutoBoolean(name, raw string) (json.RawMessage, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "auto":
		return json.RawMessage(`"auto"`), nil
	case "true":
		return json.RawMessage("true"), nil
	case "false":
		return json.RawMessage("false"), nil
	default:
		return nil, fmt.Errorf("%s 必须是 auto、true 或 false", name)
	}
}

func validateKnownAgnesLimits(kind, model, ratio, resolution, count, duration string) error {
	ratio = strings.ToLower(strings.TrimSpace(ratio))
	resolution = strings.ToUpper(strings.TrimSpace(resolution))
	count = strings.ToLower(strings.TrimSpace(count))
	duration = strings.ToLower(strings.TrimSpace(duration))
	if kind == "image" && model == defaultImageModel {
		if !stringIn(ratio, "auto", "1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3") {
			return errors.New("agnes-image 的 --ratio 仅支持 auto、1:1、4:3、3:4、16:9、9:16、3:2、2:3")
		}
		if resolution != "AUTO" && resolution != "SD" {
			return errors.New("agnes-image 的 --resolution 仅支持 auto 或 SD")
		}
		if count != "auto" && count != "1" {
			return errors.New("agnes-image 每次最多生成 1 张图，请使用 --count 1 或 auto")
		}
	}
	if kind == "video" && model == defaultVideoModel {
		if !stringIn(ratio, "auto", "9:16", "16:9", "1:1", "4:3", "3:4", "3:2", "2:3", "21:9") {
			return errors.New("agnes-video-new 的 --ratio 不支持该值")
		}
		if resolution != "AUTO" && resolution != "SD" && resolution != "HD" {
			return errors.New("agnes-video-new 的 --resolution 仅支持 auto、SD 或 HD")
		}
		if duration != "auto" {
			seconds, err := strconv.Atoi(duration)
			if err != nil || seconds < 5 || seconds > 15 {
				return errors.New("agnes-video-new 的 --duration 仅支持 auto 或 5 到 15 秒")
			}
		}
	}
	return nil
}

func stringIn(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func stringInSlice(value string, allowed []string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(candidate), value) {
			return true
		}
	}
	return false
}
