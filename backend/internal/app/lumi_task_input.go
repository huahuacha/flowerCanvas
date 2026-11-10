package app

import (
	"encoding/json"
	"strconv"
	"strings"
)

// normalizeLumiTaskInput runs before model selection and precharge. The same concrete
// specification must drive pricing and the eventual provider request.
func normalizeLumiTaskInput(input map[string]any) error {
	config, _ := input["config"].(map[string]any)
	raw, _ := config["lumiVideoParameters"].(string)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var options map[string]any
	if json.Unmarshal([]byte(raw), &options) != nil || options == nil {
		return BadAuthRequest("Lumi 参数必须为有效对象")
	}
	if options["draft_task_id"] != nil || options["omni_reference_task_type"] == "edit" || options["omni_reference_task_type"] == "extend" {
		return BadAuthRequest("本版暂不开放升清、编辑和续写；需先接入原任务规格及计费校验")
	}
	defaults := map[string]any{"duration": float64(5), "ratio": "16:9", "resolution": "720p", "generate_audio": true, "watermark": false}
	bindings := map[string]string{"duration": "videoSeconds", "ratio": "size", "resolution": "vquality", "generate_audio": "videoGenerateAudio", "watermark": "videoWatermark"}
	for key, fallback := range defaults {
		v, exists := options[key]
		if !exists {
			v = fallback
			options[key] = v
		}
		var canonical string
		switch key {
		case "duration":
			n, ok := v.(float64)
			if !ok || n != float64(int(n)) || n < 4 || n > 30 {
				return BadAuthRequest("Lumi 本版需要明确的整数时长 4～30 秒，暂不支持自动时长")
			}
			canonical = strconv.Itoa(int(n))
		case "ratio", "resolution":
			text, ok := v.(string)
			if !ok || strings.TrimSpace(text) == "" {
				return BadAuthRequest("Lumi 本版需要明确画幅与分辨率")
			}
			canonical = strings.TrimSpace(text)
			if key == "resolution" {
				canonical = normalizeLumiResolution(canonical)
				switch canonical {
				case "480p", "720p", "1080p":
				default:
					return BadAuthRequest("Lumi 分辨率仅支持 480p、720p、1080p")
				}
			} else {
				switch canonical {
				case "16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "adaptive":
				default:
					return BadAuthRequest("Lumi 画幅不受支持")
				}
			}
			options[key] = canonical
		default:
			b, ok := v.(bool)
			if !ok {
				return BadAuthRequest("Lumi 声音和水印参数必须为布尔值")
			}
			canonical = strconv.FormatBool(b)
		}
		name := bindings[key]
		if caps, ok := input["capabilityOptions"].(map[string]any); ok {
			for alias, old := range caps {
				if canonicalModelOptionName("video", alias) != name {
					continue
				}
				oldValue := providerConfigOptionValue(old)
				if name == "vquality" {
					oldValue = normalizeLumiResolution(oldValue)
				}
				if oldValue != canonical {
					return BadAuthRequest("Lumi 参数与报价规格不一致，请刷新设置后重试")
				}
				delete(caps, alias)
			}
			caps[name] = canonical
		}
		config[name] = canonical
	}
	encoded, _ := json.Marshal(options)
	config["lumiVideoParameters"] = string(encoded)
	return nil
}

func normalizeLumiResolution(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "480", "720", "1080":
		return value + "p"
	default:
		return value
	}
}
