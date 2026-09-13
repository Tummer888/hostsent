package service

import (
	"encoding/json"

	"hostsent/backend/internal/modules/uc/captcha/model"
)

// parseOverrides 解析 scene_overrides JSON；空/损坏返回 nil（按平台基线走，不报错）。
func parseOverrides(raw string) map[string]model.SceneOverride {
	if raw == "" || raw == "null" {
		return nil
	}
	out := map[string]model.SceneOverride{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// encodeOverrides 序列化 scene_overrides。
func encodeOverrides(m map[string]model.SceneOverride) string {
	if len(m) == 0 {
		return ""
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(raw)
}
