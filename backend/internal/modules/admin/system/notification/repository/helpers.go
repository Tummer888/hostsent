package repository

import (
	"encoding/json"
	"strings"
	"time"
)

// timeNow 便于测试替换的时间源。
var timeNow = time.Now

// channelScenesMatch 判断渠道启用场景是否覆盖目标场景。
// 场景数组为空视为「未限制」（渠道可用于任何场景）。
func channelScenesMatch(rawScenes, scene string) bool {
	raw := strings.TrimSpace(rawScenes)
	if raw == "" || raw == "null" || raw == "[]" {
		return true
	}
	var scenes []string
	if err := json.Unmarshal([]byte(raw), &scenes); err != nil {
		return true
	}
	if len(scenes) == 0 {
		return true
	}
	for _, s := range scenes {
		if s == scene {
			return true
		}
	}
	return false
}
