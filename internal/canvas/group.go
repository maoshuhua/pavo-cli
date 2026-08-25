package canvas

import (
	"errors"
	"fmt"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

const DefaultGroupPadding = 32

type GroupOptions struct {
	NodeKey     string
	Name        string
	ModeCode    string
	BorderColor string
	FillColor   string
	Padding     float64
}

func GroupNodes(detail *api.CanvasProjectDetail, references []string, options GroupOptions) (string, []string, error) {
	if len(references) < 2 {
		return "", nil, errors.New("打组至少需要两个节点")
	}
	padding := options.Padding
	if padding <= 0 {
		padding = DefaultGroupPadding
	}
	selected := []string{}
	seen := map[string]bool{}
	for _, reference := range references {
		node, err := FindNode(detail, reference)
		if err != nil {
			return "", nil, err
		}
		if !seen[node.NodeKey] {
			selected = append(selected, node.NodeKey)
			seen[node.NodeKey] = true
		}
	}
	groupsToRemove := map[string]bool{}
	members := map[string]bool{}
	var collectGroup func(string)
	collectGroup = func(groupKey string) {
		groupsToRemove[groupKey] = true
		for _, node := range detail.NodeList {
			if node.ParentKey != groupKey {
				continue
			}
			if string(node.NodeType) == "group" {
				collectGroup(node.NodeKey)
			} else {
				members[node.NodeKey] = true
			}
		}
	}
	for _, key := range selected {
		node := nodeByKey(detail, key)
		if node == nil {
			continue
		}
		if string(node.NodeType) == "group" {
			collectGroup(key)
		} else {
			members[key] = true
		}
	}
	if len(members) < 2 {
		return "", nil, errors.New("拆平已有分组后，打组至少需要两个普通节点")
	}

	type boundsItem struct {
		key                 string
		x, y, width, height float64
	}
	items := make([]boundsItem, 0, len(members))
	minX, minY := 1e30, 1e30
	maxX, maxY := -1e30, -1e30
	for key := range members {
		node := nodeByKey(detail, key)
		x, y, err := AbsolutePosition(detail, key)
		if err != nil {
			return "", nil, err
		}
		width := numericScalar(node.Measured.Width, 280)
		height := numericScalar(node.Measured.Height, 280)
		items = append(items, boundsItem{key: key, x: x, y: y, width: width, height: height})
		if x < minX {
			minX = x
		}
		if y < minY {
			minY = y
		}
		if x+width > maxX {
			maxX = x + width
		}
		if y+height > maxY {
			maxY = y + height
		}
	}
	for groupKey := range groupsToRemove {
		if err := DeleteNodeFromDetail(detail, groupKey); err != nil {
			return "", nil, err
		}
	}
	groupKey := strings.TrimSpace(options.NodeKey)
	if groupKey == "" {
		var err error
		groupKey, err = NewNodeKey("group", "")
		if err != nil {
			return "", nil, err
		}
	}
	name := strings.TrimSpace(options.Name)
	if name == "" {
		name = defaultNodeName("group", detail)
	}
	border := strings.TrimSpace(options.BorderColor)
	if border == "" {
		border = "#0ABCCF"
	}
	fill := strings.TrimSpace(options.FillColor)
	if fill == "" {
		fill = "#FBFBFB1A"
	}
	data := map[string]any{"manualBounds": false, "color": map[string]any{"border": border, "fill": fill}}
	if strings.TrimSpace(options.ModeCode) != "" {
		data["modeCode"] = strings.TrimSpace(options.ModeCode)
	}
	groupX, groupY := minX-padding, minY-padding
	width, height := maxX-minX+2*padding, maxY-minY+2*padding
	item, err := NewNode(detail, NewNodeOptions{NodeKey: groupKey, Type: "group", Name: name, X: &groupX, Y: &groupY, Width: width, Height: height, Data: data})
	if err != nil {
		return "", nil, err
	}
	detail.NodeList = append(detail.NodeList, NodeFromWriteItem(*item))
	memberKeys := make([]string, 0, len(items))
	for _, member := range items {
		node := nodeByKey(detail, member.key)
		if err := SetNodeParent(node, groupKey, member.x-groupX, member.y-groupY); err != nil {
			return "", nil, err
		}
		memberKeys = append(memberKeys, member.key)
	}
	return groupKey, memberKeys, nil
}

func UngroupNode(detail *api.CanvasProjectDetail, reference string) (string, []string, error) {
	group, err := FindNode(detail, reference)
	if err != nil {
		return "", nil, err
	}
	if string(group.NodeType) != "group" {
		return "", nil, fmt.Errorf("节点 %s 不是 group", group.NodeKey)
	}
	groupX, groupY, err := AbsolutePosition(detail, group.NodeKey)
	if err != nil {
		return "", nil, err
	}
	parentKey := group.ParentKey
	children := []string{}
	for index := range detail.NodeList {
		node := &detail.NodeList[index]
		if node.ParentKey != group.NodeKey {
			continue
		}
		x := numericScalar(node.Position.PositionX, 0) + groupX
		y := numericScalar(node.Position.PositionY, 0) + groupY
		if parentKey != "" {
			parentX, parentY, parentErr := AbsolutePosition(detail, parentKey)
			if parentErr != nil {
				return "", nil, parentErr
			}
			x -= parentX
			y -= parentY
		}
		if err := SetNodeParent(node, parentKey, x, y); err != nil {
			return "", nil, err
		}
		children = append(children, node.NodeKey)
	}
	groupKey := group.NodeKey
	if err := DeleteNodeFromDetail(detail, groupKey); err != nil {
		return "", nil, err
	}
	return groupKey, children, nil
}
