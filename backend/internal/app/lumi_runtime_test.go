package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"yingce/backend/internal/protocol"
)

type lumiInlineTestAdapter struct{ protocol.Adapter }

func TestLumiWorkbenchCarriesOptionsIntoTaskInput(t *testing.T) {
	const options = `{"draft":true,"resolution":"480p","duration":8,"seed":0,"generate_audio":false}`
	service := &Service{}
	config, err := service.resolveWorkbenchProviderConfig("user", WorkbenchKindVideo, "text_to_video", "seedance-2.5", "logical-lumi", "",
		map[string]any{"lumiVideoParameters": options}, nil)
	if err != nil {
		t.Fatal(err)
	}
	input, err := normalizeTaskInput(map[string]any{"config": config})
	if err != nil {
		t.Fatal(err)
	}
	admitted := input["config"].(map[string]any)
	if admitted["videoSeconds"] != "8" || admitted["vquality"] != "480p" || admitted["videoGenerateAudio"] != "false" {
		t.Fatalf("workbench options did not reach admission: %#v", admitted)
	}
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(admitted["lumiVideoParameters"].(string)), &snapshot); err != nil || snapshot["seed"] != float64(0) || snapshot["draft"] != true {
		t.Fatalf("workbench options = %#v, error = %v", snapshot, err)
	}
}

func TestLumiPackageRegistersVideoAdapter(t *testing.T) {
	center, err := newPluginRuntime(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := center.registrySnapshot().Get("lumi-ark-video"); !ok {
		t.Fatal("bundled Lumi video adapter is not registered")
	}
}

func (a lumiInlineTestAdapter) Metadata() protocol.Metadata {
	m := a.Adapter.Metadata()
	m.DetachedPolling = false
	return m
}

func TestLumiRuntimeHTTP(t *testing.T) {
	allowLoopbackProviderTest(t)
	raw, e := os.ReadFile(filepath.Join("..", "..", "..", "plugin-packages", "lumi-ark-video.yingce-plugin"))
	if e != nil {
		t.Fatal(e)
	}
	pkg, e := protocol.ParsePluginPackage(raw)
	if e != nil {
		t.Fatal(e)
	}
	adapters, e := protocol.LoadInstalledProviders(pkg.ManifestRaw, nil)
	if e != nil {
		t.Fatal(e)
	}
	var server *httptest.Server
	creates, polls, downloads := 0, 0, 0
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/contents/generations/tasks":
			creates++
			if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer fake-local-key" || r.Header.Get("Idempotency-Key") == "" {
				t.Error("missing create method/auth/idempotency")
			}
			var b map[string]any
			if json.NewDecoder(r.Body).Decode(&b) != nil {
				t.Error("invalid body")
			}
			if b["duration"] != float64(5) || b["generate_audio"] != false || b["seed"] != float64(0) {
				t.Errorf("bad fields %#v", b)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"task_platform_mock","status":"queued"}`))
		case "/api/v3/contents/generations/tasks/task_platform_mock":
			polls++
			w.Header().Set("Content-Type", "application/json")
			if polls == 1 {
				w.Write([]byte(`{"status":"running"}`))
				return
			}
			w.Write([]byte(`{"id":"task_platform_mock","status":"succeeded","relay_video_url":"` + server.URL + `/video.mp4","usage":{"total_tokens":10}}`))
		case "/video.mp4":
			downloads++
			w.Header().Set("Content-Type", "video/mp4")
			w.Write([]byte("local video fixture"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cfg := providerConfig{BaseURL: server.URL, APIKey: "fake-local-key", Model: "seedance-2.5", InterfaceType: "lumi-ark-video", LumiVideoParameters: `{"duration":5,"generate_audio":false,"seed":0}`}
	result, e := runProtocolAdapterTaskWithPolicy(context.Background(), canvasGenerationInput{Mode: "video", Prompt: "local test", Config: cfg}, lumiInlineTestAdapter{adapters[0]}, fastVideoPollPolicy())
	if e != nil {
		t.Fatal(e)
	}
	if result["mode"] != "video" || creates != 1 || polls != 2 || downloads != 1 {
		t.Fatalf("result=%#v counts=%d/%d/%d", result, creates, polls, downloads)
	}
}

func TestLumiAdmissionUsesPricedSpecification(t *testing.T) {
	input := map[string]any{"config": map[string]any{"lumiVideoParameters": `{"duration":12,"resolution":"1080p","ratio":"16:9","generate_audio":false}`}}
	normalized, e := normalizeTaskInput(input)
	if e != nil {
		t.Fatal(e)
	}
	config := normalized["config"].(map[string]any)
	if config["videoSeconds"] != "12" || config["vquality"] != "1080p" || config["videoGenerateAudio"] != "false" {
		t.Fatalf("projection %#v", config)
	}
	for _, raw := range []string{`{"duration":-1}`, `{"draft_task_id":"task_sample"}`, `[]`, `{"duration":5.5}`} {
		_, e = normalizeTaskInput(map[string]any{"config": map[string]any{"lumiVideoParameters": raw}})
		if e == nil {
			t.Errorf("unsafe specification accepted: %s", raw)
		}
	}
	_, e = normalizeTaskInput(map[string]any{"config": map[string]any{"lumiVideoParameters": `{"duration":12}`}, "capabilityOptions": map[string]any{"videoSeconds": 5}})
	if e == nil {
		t.Fatal("conflicting price accepted")
	}
}

func TestLumiAdmissionNormalizesResolution(t *testing.T) {
	input := map[string]any{"config": map[string]any{"lumiVideoParameters": `{"resolution":"720"}`}}
	normalized, err := normalizeTaskInput(input)
	if err != nil {
		t.Fatal(err)
	}
	if normalized["config"].(map[string]any)["vquality"] != "720p" {
		t.Fatal("legacy resolution was not normalized")
	}
	for _, raw := range []string{`{"resolution":"banana"}`, `{"ratio":"banana"}`} {
		if _, err := normalizeTaskInput(map[string]any{"config": map[string]any{"lumiVideoParameters": raw}}); err == nil {
			t.Fatalf("invalid enum accepted: %s", raw)
		}
	}
}

func TestLumiAdmissionRejectsConflictingAliases(t *testing.T) {
	for _, caps := range []map[string]any{{"duration": 5}, {"resolution": "480"}, {"aspectRatio": "9:16"}} {
		_, err := normalizeTaskInput(map[string]any{"config": map[string]any{"lumiVideoParameters": `{"duration":12,"resolution":"720p","ratio":"16:9"}`}, "capabilityOptions": caps})
		if err == nil {
			t.Fatalf("conflicting alias accepted: %#v", caps)
		}
	}
}
