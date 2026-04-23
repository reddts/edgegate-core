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
	if static.Box == nil {
		return nil
	}

	cacheFile := service.FromContext[adapter.CacheFile](static.Box.Context())
	outbounds := static.Box.GetInstance().Outbound().Outbounds()
	outboundsConverted := make(map[string]*OutboundInfo, len(outbounds))
	var outboundGroups []adapter.OutboundGroup
	for _, it := range outbounds {
		if outGroup, isGroup := it.(adapter.OutboundGroup); isGroup {
			outboundGroups = append(outboundGroups, outGroup)
		}
		outboundsConverted[it.Tag()] = GetProxyInfo(it)
	}

	var groups OutboundGroupList
	for _, outboundGroup := range outboundGroups {
		var groupInfo OutboundGroup
		groupInfo.Tag = outboundGroup.Tag()
		groupInfo.Type = outboundGroup.Type()
		_, groupInfo.Selectable = outboundGroup.(*protocolgroup.Selector)
		selectedTag := outboundGroup.Now()
		groupInfo.Selected = outboundsConverted[selectedTag]
		outboundsConverted[outboundGroup.Tag()].GroupSelectedOutbound = groupInfo.Selected
		if cacheFile != nil {
			if isExpand, loaded := cacheFile.LoadGroupExpand(groupInfo.Tag); loaded {
				groupInfo.IsExpand = isExpand
			}
		}

		for _, itemTag := range outboundGroup.All() {
			if onlyGroupItems && itemTag != selectedTag {
				continue
			}
			pinfo := outboundsConverted[itemTag]
			if pinfo == nil {
				continue
			}
			pinfo.IsSelected = itemTag == selectedTag
			groupInfo.Items = append(groupInfo.Items, pinfo)
			pinfo.IsVisible = !strings.Contains(itemTag, "§hide§")
			pinfo.TagDisplay = TrimTagName(itemTag)
		}
		if len(groupInfo.Items) == 0 {
			continue
		}
		groups.Items = append(groups.Items, &groupInfo)
	}

	return &groups
}

func TrimTagName(tag string) string {
	return strings.Trim(strings.Split(tag, "§")[0], " ")
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
