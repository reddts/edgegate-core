package hcore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	protocolgroup "github.com/sagernet/sing-box/protocol/group"
	"github.com/sagernet/sing/service"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

func GetProxyInfo(detour adapter.Outbound) *OutboundInfo {
	if static.Box == nil {
		return nil
	}
	out := &OutboundInfo{
		Tag:  detour.Tag(),
		Type: detour.Type(),
	}
	urlTestHistory := static.Box.UrlTestHistory().LoadURLTestHistory(detour.Tag())
	if urlTestHistory != nil {
		out.UrlTestTime = timestamppb.New(urlTestHistory.Time)
		out.UrlTestDelay = int32(urlTestHistory.Delay)
		if _, isGroup := detour.(adapter.OutboundGroup); isGroup {
			out.IsGroup = true
		}
	}

	return out
}

func GetAllProxiesInfo(onlyGroupItems bool) *OutboundGroupList {
	all, main := GetAllProxiesSnapshots()
	if onlyGroupItems {
		return main
	}
	return all
}

func GetAllProxiesSnapshots() (*OutboundGroupList, *OutboundGroupList) {
	if static.Box == nil {
		return nil, nil
	}

	cacheFile := service.FromContext[adapter.CacheFile](static.Box.Context())
	outbounds := static.Box.GetInstance().Outbound().Outbounds()
	baseOutbounds := make(map[string]*OutboundInfo, len(outbounds))
	var outboundGroups []adapter.OutboundGroup
	for _, it := range outbounds {
		if outGroup, isGroup := it.(adapter.OutboundGroup); isGroup {
			outboundGroups = append(outboundGroups, outGroup)
		}
		info := GetProxyInfo(it)
		if info == nil {
			continue
		}
		baseOutbounds[it.Tag()] = info
	}

	allGroups := &OutboundGroupList{}
	mainGroups := &OutboundGroupList{}
	for _, outboundGroup := range outboundGroups {
		allGroup := newOutboundGroupInfo(outboundGroup, cacheFile)
		mainGroup := newOutboundGroupInfo(outboundGroup, cacheFile)
		selectedTag := outboundGroup.Now()
		selectedInfo := decorateOutboundInfo(baseOutbounds[selectedTag], true)
		allGroup.Selected = selectedInfo
		mainGroup.Selected = cloneOutboundInfo(selectedInfo)
		if groupBase := baseOutbounds[outboundGroup.Tag()]; groupBase != nil {
			groupBase.GroupSelectedOutbound = cloneOutboundInfo(selectedInfo)
		}

		for _, itemTag := range outboundGroup.All() {
			itemInfo := decorateOutboundInfo(baseOutbounds[itemTag], itemTag == selectedTag)
			if itemInfo == nil {
				continue
			}
			allGroup.Items = append(allGroup.Items, itemInfo)
			if itemTag == selectedTag {
				mainGroup.Items = append(mainGroup.Items, cloneOutboundInfo(itemInfo))
			}
		}
		if len(allGroup.Items) == 0 {
			continue
		}
		allGroups.Items = append(allGroups.Items, &allGroup)
		if len(mainGroup.Items) > 0 {
			mainGroups.Items = append(mainGroups.Items, &mainGroup)
		}
	}

	return allGroups, mainGroups
}

func newOutboundGroupInfo(
	outboundGroup adapter.OutboundGroup,
	cacheFile adapter.CacheFile,
) OutboundGroup {
	var groupInfo OutboundGroup
	groupInfo.Tag = outboundGroup.Tag()
	groupInfo.Type = outboundGroup.Type()
	_, groupInfo.Selectable = outboundGroup.(*protocolgroup.Selector)
	if cacheFile != nil {
		if isExpand, loaded := cacheFile.LoadGroupExpand(groupInfo.Tag); loaded {
			groupInfo.IsExpand = isExpand
		}
	}
	return groupInfo
}

func decorateOutboundInfo(base *OutboundInfo, isSelected bool) *OutboundInfo {
	if base == nil {
		return nil
	}
	info := cloneOutboundInfo(base)
	info.IsSelected = isSelected
	info.IsVisible = !strings.Contains(info.Tag, "搂hide搂")
	info.TagDisplay = TrimTagName(info.Tag)
	if info.GroupSelectedOutbound != nil {
		info.GroupSelectedOutbound = cloneOutboundInfo(info.GroupSelectedOutbound)
	}
	return info
}

func TrimTagName(tag string) string {
	return strings.Trim(strings.Split(tag, "搂")[0], " ")
}

type outboundStream interface {
	Send(*OutboundGroupList) error
	Context() context.Context
}

func allProxiesInfoStream(stream outboundStream, onlyMain bool) error {
	if static.Box == nil {
		return fmt.Errorf("core service is not started")
	}

	if info := GetAllProxiesInfo(onlyMain); info != nil {
		stream.Send(info)
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			if info := GetAllProxiesInfo(onlyMain); info != nil {
				stream.Send(info)
			}
		}
	}
}
