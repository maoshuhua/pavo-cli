package canvas

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

type EdgeOptions struct {
	ConnectionID   string
	SourceHandle   string
	TargetHandle   string
	SourcePortType string
	TargetPortType string
	Role           string
	MediaOrder     int
	ConnectionType string
	ColorKey       string
	Selectable     *bool
	Deletable      *bool
	Style          json.RawMessage
}

func CloneDetail(detail *api.CanvasProjectDetail) (*api.CanvasProjectDetail, error) {
	if detail == nil {
		return nil, errors.New("画布详情为空")
	}
	data, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("复制画布详情失败: %w", err)
	}
	var clone api.CanvasProjectDetail
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&clone); err != nil {
		return nil, fmt.Errorf("复制画布详情失败: %w", err)
	}
	return &clone, nil
}

func NodeFromWriteItem(item api.CanvasBatchNodeWriteItem) api.CanvasNode {
	return api.CanvasNode{
		NodeKey:   item.NodeKey,
		NodeType:  api.ScalarString(item.Type),
		Name:      item.Name,
		Position:  api.CanvasNodePosition{PositionX: api.ScalarString(item.Position.PositionX), PositionY: api.ScalarString(item.Position.PositionY)},
		Measured:  api.CanvasNodeMeasured{Width: api.ScalarString(item.Measured.Width), Height: api.ScalarString(item.Measured.Height)},
		ParentKey: item.ParentKey,
		Data:      json.RawMessage(item.Data),
	}
}

func nodeIndex(detail *api.CanvasProjectDetail, nodeKey string) int {
	for index := range detail.NodeList {
		if detail.NodeList[index].NodeKey == nodeKey {
			return index
		}
	}
	return -1
}

func nodeByKey(detail *api.CanvasProjectDetail, nodeKey string) *api.CanvasNode {
	if index := nodeIndex(detail, nodeKey); index >= 0 {
		return &detail.NodeList[index]
	}
	return nil
}

func numericScalar(value api.ScalarString, fallback float64) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(string(value)), 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func AbsolutePosition(detail *api.CanvasProjectDetail, nodeKey string) (float64, float64, error) {
	visiting := map[string]bool{}
	var resolve func(string) (float64, float64, error)
	resolve = func(key string) (float64, float64, error) {
		node := nodeByKey(detail, key)
		if node == nil {
			return 0, 0, fmt.Errorf("找不到节点 %q", key)
		}
		if visiting[key] {
			return 0, 0, fmt.Errorf("节点 parent_key 形成环: %s", key)
		}
		visiting[key] = true
		defer delete(visiting, key)
		x := numericScalar(node.Position.PositionX, 0)
		y := numericScalar(node.Position.PositionY, 0)
		if strings.TrimSpace(node.ParentKey) == "" {
			return x, y, nil
		}
		parentX, parentY, err := resolve(node.ParentKey)
		if err != nil {
			return 0, 0, err
		}
		return x + parentX, y + parentY, nil
	}
	return resolve(nodeKey)
}

func SetNodeParent(node *api.CanvasNode, parentKey string, x, y float64) error {
	if node == nil {
		return errors.New("节点为空")
	}
	data, err := NodeData(*node)
	if err != nil {
		return err
	}
	parentKey = strings.TrimSpace(parentKey)
	if parentKey == "" {
		delete(data, "parent_key")
	} else {
		data["parent_key"] = parentKey
	}
	node.ParentKey = parentKey
	node.Position.PositionX = api.ScalarString(formatNumber(x))
	node.Position.PositionY = api.ScalarString(formatNumber(y))
	encoded, err := EncodeObject(data)
	if err != nil {
		return err
	}
	node.Data = json.RawMessage(encoded)
	return nil
}

func defaultBool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func ConnectionWriteItem(connection api.CanvasConnection) api.CanvasBatchConnectionWriteItem {
	connectionType := firstString(connection.ConnectionType, connection.Type, "default")
	style := connection.StyleJSON
	if len(bytes.TrimSpace(style)) == 0 {
		style = connection.Style
	}
	mediaOrder := 0
	if len(bytes.TrimSpace(connection.MediaOrder)) > 0 {
		_ = json.Unmarshal(connection.MediaOrder, &mediaOrder)
	}
	return api.CanvasBatchConnectionWriteItem{
		ConnectionID: connection.ConnectionID, Source: connection.SourceNodeKey, Target: connection.TargetNodeKey,
		SourceHandle: firstString(connection.SourceHandle, "source"), TargetHandle: firstString(connection.TargetHandle, "target"),
		SourcePortType: connection.SourcePortType, TargetPortType: connection.TargetPortType,
		Role: connection.Role, MediaOrder: mediaOrder, ConnectionType: connectionType, ColorKey: connection.ColorKey,
		Selectable: defaultBool(connection.Selectable, true), Deletable: defaultBool(connection.Deletable, true), Style: style,
	}
}

func AddConnection(detail *api.CanvasProjectDetail, sourceRef, targetRef string, options EdgeOptions) (string, error) {
	source, err := FindNode(detail, sourceRef)
	if err != nil {
		return "", fmt.Errorf("解析 source 失败: %w", err)
	}
	target, err := FindNode(detail, targetRef)
	if err != nil {
		return "", fmt.Errorf("解析 target 失败: %w", err)
	}
	if source.NodeKey == target.NodeKey {
		return "", errors.New("不能连接节点自身")
	}
	for _, connection := range detail.ConnectionList {
		if connection.SourceNodeKey == source.NodeKey && connection.TargetNodeKey == target.NodeKey {
			return "", fmt.Errorf("连线已存在: %s", connection.ConnectionID)
		}
	}
	connectionID := strings.TrimSpace(options.ConnectionID)
	if connectionID == "" {
		connectionID, err = NewConnectionID(string(source.NodeType), NodeMediaType(*source), string(target.NodeType), NodeMediaType(*target))
		if err != nil {
			return "", err
		}
	}
	for _, connection := range detail.ConnectionList {
		if connection.ConnectionID == connectionID {
			return "", fmt.Errorf("connection_id %q 已存在", connectionID)
		}
	}
	sourceData, err := NodeData(*source)
	if err != nil {
		return "", err
	}
	targetData, err := NodeData(*target)
	if err != nil {
		return "", err
	}
	AddUniqueString(sourceData, "target", target.NodeKey)
	AddUniqueString(targetData, "source", source.NodeKey)
	sourceEncoded, _ := EncodeObject(sourceData)
	targetEncoded, _ := EncodeObject(targetData)
	source.Data = json.RawMessage(sourceEncoded)
	target.Data = json.RawMessage(targetEncoded)
	selectable := defaultBool(options.Selectable, true)
	deletable := defaultBool(options.Deletable, true)
	mediaOrder, _ := json.Marshal(options.MediaOrder)
	detail.ConnectionList = append(detail.ConnectionList, api.CanvasConnection{
		ConnectionID: connectionID, SourceNodeKey: source.NodeKey, TargetNodeKey: target.NodeKey,
		SourceHandle: firstString(options.SourceHandle, "source"), TargetHandle: firstString(options.TargetHandle, "target"),
		SourcePortType: options.SourcePortType, TargetPortType: options.TargetPortType, Role: options.Role,
		MediaOrder: mediaOrder, ConnectionType: firstString(options.ConnectionType, "default"), ColorKey: options.ColorKey,
		Selectable: &selectable, Deletable: &deletable, StyleJSON: append(json.RawMessage(nil), options.Style...),
	})
	return connectionID, nil
}

func DeleteConnection(detail *api.CanvasProjectDetail, connectionID string) error {
	connectionID = strings.TrimSpace(connectionID)
	index := -1
	var matched api.CanvasConnection
	for current := range detail.ConnectionList {
		if detail.ConnectionList[current].ConnectionID == connectionID {
			index = current
			matched = detail.ConnectionList[current]
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("找不到连线 %q", connectionID)
	}
	for _, mutation := range []struct{ key, field, value string }{
		{matched.SourceNodeKey, "target", matched.TargetNodeKey},
		{matched.TargetNodeKey, "source", matched.SourceNodeKey},
	} {
		node := nodeByKey(detail, mutation.key)
		if node == nil {
			continue
		}
		data, err := NodeData(*node)
		if err != nil {
			return err
		}
		RemoveString(data, mutation.field, mutation.value)
		encoded, _ := EncodeObject(data)
		node.Data = json.RawMessage(encoded)
	}
	detail.ConnectionList = append(detail.ConnectionList[:index], detail.ConnectionList[index+1:]...)
	return nil
}

func DeleteNodeFromDetail(detail *api.CanvasProjectDetail, nodeKey string) error {
	index := nodeIndex(detail, nodeKey)
	if index < 0 {
		return fmt.Errorf("找不到节点 %q", nodeKey)
	}
	connectionIDs := []string{}
	for _, connection := range detail.ConnectionList {
		if connection.SourceNodeKey == nodeKey || connection.TargetNodeKey == nodeKey {
			connectionIDs = append(connectionIDs, connection.ConnectionID)
		}
	}
	for _, connectionID := range connectionIDs {
		if err := DeleteConnection(detail, connectionID); err != nil {
			return err
		}
	}
	detail.NodeList = append(detail.NodeList[:index], detail.NodeList[index+1:]...)
	return nil
}

func DiffDetails(before, after *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
	request := NewBatchRequest()
	beforeNodes := map[string]api.CanvasNode{}
	afterNodes := map[string]api.CanvasNode{}
	for _, node := range before.NodeList {
		beforeNodes[node.NodeKey] = node
	}
	for _, node := range after.NodeList {
		afterNodes[node.NodeKey] = node
	}
	keys := make([]string, 0, len(beforeNodes)+len(afterNodes))
	seen := map[string]bool{}
	for key := range beforeNodes {
		keys = append(keys, key)
		seen[key] = true
	}
	for key := range afterNodes {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		oldNode, oldOK := beforeNodes[key]
		newNode, newOK := afterNodes[key]
		switch {
		case !oldOK && newOK:
			newData, err := NodeData(newNode)
			if err != nil {
				return nil, err
			}
			item, err := WriteItemFromNode(newNode, newData)
			if err != nil {
				return nil, err
			}
			request.Nodes.Create = append(request.Nodes.Create, item)
		case oldOK && !newOK:
			request.Nodes.Delete = append(request.Nodes.Delete, key)
		case oldOK && newOK:
			oldData, err := NodeData(oldNode)
			if err != nil {
				return nil, err
			}
			newData, err := NodeData(newNode)
			if err != nil {
				return nil, err
			}
			oldItem, err := WriteItemFromNode(oldNode, oldData)
			if err != nil {
				return nil, err
			}
			newItem, err := WriteItemFromNode(newNode, newData)
			if err != nil {
				return nil, err
			}
			if !reflect.DeepEqual(oldItem, newItem) {
				request.Nodes.Update = append(request.Nodes.Update, newItem)
			}
		}
	}
	beforeConnections := map[string]api.CanvasConnection{}
	afterConnections := map[string]api.CanvasConnection{}
	for _, connection := range before.ConnectionList {
		beforeConnections[connection.ConnectionID] = connection
	}
	for _, connection := range after.ConnectionList {
		afterConnections[connection.ConnectionID] = connection
	}
	connectionIDs := make([]string, 0, len(beforeConnections)+len(afterConnections))
	seen = map[string]bool{}
	for id := range beforeConnections {
		connectionIDs = append(connectionIDs, id)
		seen[id] = true
	}
	for id := range afterConnections {
		if !seen[id] {
			connectionIDs = append(connectionIDs, id)
		}
	}
	sort.Strings(connectionIDs)
	for _, id := range connectionIDs {
		oldConnection, oldOK := beforeConnections[id]
		newConnection, newOK := afterConnections[id]
		switch {
		case !oldOK && newOK:
			request.Connections.Create = append(request.Connections.Create, ConnectionWriteItem(newConnection))
		case oldOK && !newOK:
			request.Connections.Delete = append(request.Connections.Delete, api.CanvasBatchConnectionDeleteItem{ConnectionID: id})
		case oldOK && newOK:
			if !reflect.DeepEqual(ConnectionWriteItem(oldConnection), ConnectionWriteItem(newConnection)) {
				request.Connections.Delete = append(request.Connections.Delete, api.CanvasBatchConnectionDeleteItem{ConnectionID: id})
				request.Connections.Create = append(request.Connections.Create, ConnectionWriteItem(newConnection))
			}
		}
	}
	return request, nil
}

func BatchRequestEmpty(request *api.CanvasBatchRequest) bool {
	return request == nil || (len(request.Nodes.Create) == 0 && len(request.Nodes.Update) == 0 && len(request.Nodes.Delete) == 0 && len(request.Connections.Create) == 0 && len(request.Connections.Delete) == 0)
}
