package canvas

import (
	"net/url"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

// BuildURL builds the browser route used by pavo-app-front. The numeric
// project ID belongs in the path; the project and canvas UUIDs are query
// parameters and are not interchangeable with it.
func BuildURL(appBaseURL, projectID, projectUUID, canvasUUID string) string {
	appBaseURL = strings.TrimRight(strings.TrimSpace(appBaseURL), "/")
	projectID = strings.TrimSpace(projectID)
	projectUUID = strings.TrimSpace(projectUUID)
	canvasUUID = strings.TrimSpace(canvasUUID)
	if appBaseURL == "" || projectID == "" || projectUUID == "" || canvasUUID == "" {
		return ""
	}
	base, err := url.Parse(appBaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return ""
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/canvas/" + url.PathEscape(projectID)
	query := base.Query()
	query.Set("canvas_uuid", canvasUUID)
	query.Set("project_uuid", projectUUID)
	base.RawQuery = query.Encode()
	return base.String()
}

func ProjectIDFromDetail(detail *api.CanvasProjectDetail) string {
	if detail == nil {
		return ""
	}
	if value := strings.TrimSpace(string(detail.CurrentCanvas.ProjectID)); value != "" {
		return value
	}
	return strings.TrimSpace(string(detail.ProjectMeta.ID))
}

func ProjectIDFromCreated(created *api.CanvasProjectCreated) string {
	if created == nil {
		return ""
	}
	if value := strings.TrimSpace(string(created.Canvas.ProjectID)); value != "" {
		return value
	}
	return strings.TrimSpace(string(created.Project.ID))
}

func ProjectIDFromEntry(entry api.CanvasProjectEntry) string {
	if value := strings.TrimSpace(string(entry.ProjectID)); value != "" {
		return value
	}
	return strings.TrimSpace(string(entry.ID))
}

func CanvasUUIDFromEntry(entry api.CanvasProjectEntry) string {
	if value := strings.TrimSpace(entry.CanvasUUID); value != "" {
		return value
	}
	return strings.TrimSpace(entry.LatestCanvasUUID)
}
