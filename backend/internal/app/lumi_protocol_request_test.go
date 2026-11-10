package app

import (
	"encoding/json"
	"testing"
)

func TestLumiConfigReachesProtocol(t *testing.T) {
	var cfg providerConfig
	if err := json.Unmarshal([]byte(`{"interfaceType":"lumi-ark-video","lumiVideoParameters":"{\"seed\":0,\"generate_audio\":false,\"duration\":-1}"}`), &cfg); err != nil {
		t.Fatal(err)
	}
	r := protocolRequestFromInput(canvasGenerationInput{Mode: "video", Config: cfg})
	o := r.ProviderOptions["lumi-ark-video"]
	if o["seed"] != float64(0) || o["generate_audio"] != false || o["duration"] != float64(-1) {
		t.Fatalf("lost configured fields: %#v", o)
	}
}

func TestLumiIgnoresUnadmittedMetadataOptions(t *testing.T) {
	r := protocolRequestFromInput(canvasGenerationInput{Mode: "video", Config: providerConfig{InterfaceType: "lumi-ark-video", VideoSeconds: "5", VQuality: "720"}, Metadata: map[string]any{"providerOptions": map[string]any{"lumi-ark-video": map[string]any{"draft_task_id": "task_bypass", "omni_reference_task_type": "edit", "duration": 30, "resolution": "1080p"}}}})
	o := r.ProviderOptions["lumi-ark-video"]
	if o["draft_task_id"] != nil || o["omni_reference_task_type"] != nil || o["duration"] != 5 || o["resolution"] != "720p" {
		t.Fatalf("unadmitted metadata reached provider: %#v", o)
	}
}
