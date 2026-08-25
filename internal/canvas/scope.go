package canvas

import (
	"errors"
	"os"
	"strings"
)

type Scope struct {
	ProjectUUID string `json:"project_uuid"`
	CanvasUUID  string `json:"canvas_uuid,omitempty"`
	SessionID   string `json:"session_id"`
	BindingPath string `json:"binding_path,omitempty"`
}

// ResolveScope combines explicit flags with the nearest workspace binding.
// A binding's canvas is reused only when it belongs to the selected project.
func ResolveScope(directory, explicitProject, explicitCanvas string) (Scope, error) {
	explicitProject = strings.TrimSpace(explicitProject)
	explicitCanvas = strings.TrimSpace(explicitCanvas)
	binding, path, err := FindBinding(directory)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Scope{}, err
	}

	scope := Scope{ProjectUUID: explicitProject, CanvasUUID: explicitCanvas}
	if binding != nil {
		if scope.ProjectUUID == "" {
			scope.ProjectUUID = binding.ProjectUUID
		}
		if scope.CanvasUUID == "" && scope.ProjectUUID == binding.ProjectUUID {
			scope.CanvasUUID = binding.CanvasUUID
		}
		if scope.ProjectUUID == binding.ProjectUUID {
			scope.SessionID = binding.SessionID
			scope.BindingPath = path
		}
	}
	if scope.ProjectUUID == "" {
		return Scope{}, errors.New("缺少画布项目；请传 --project 或先运行 pavo canvas use")
	}
	if scope.SessionID == "" {
		scope.SessionID, err = RandomUUID()
		if err != nil {
			return Scope{}, err
		}
	}
	return scope, nil
}
