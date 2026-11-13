package protocol

import (
	"context"
	"testing"
)

func TestLumiVideoRequestContract(t *testing.T) {
	a := officialPackageAdapter(t, "lumi-ark-video.yingce-plugin", "lumi-ark-video")
	r := GenerationRequest{Model: "seedance-2.5", Prompt: "a cat", Extra: map[string]any{"idempotencyKey": "one-order"}, ProviderOptions: map[string]map[string]any{"lumi-ark-video": {"seed": 0, "priority": 0, "generate_audio": false, "watermark": false, "duration": -1}}}
	spec, err := a.BuildCreate(context.Background(), RequestContext{Request: r})
	if err != nil {
		t.Fatal(err)
	}
	b := manifestTestBody(t, spec)
	if spec.Path != "/api/v3/contents/generations/tasks" || spec.Headers["Idempotency-Key"] != "one-order" {
		t.Fatalf("request: %#v", spec)
	}
	for k, v := range map[string]any{"seed": float64(0), "priority": float64(0), "generate_audio": false, "watermark": false, "duration": float64(-1)} {
		if b[k] != v {
			t.Errorf("%s = %#v, want %#v", k, b[k], v)
		}
	}
	if _, ok := b["ratio"]; ok {
		t.Fatal("must not inject ratio default")
	}
	if _, ok := b["resolution"]; ok {
		t.Fatal("must not inject resolution default")
	}
	if _, ok := b["idempotencyKey"]; ok {
		t.Fatal("host key leaked into body")
	}
}

func TestLumiVideoModes(t *testing.T) {
	a := officialPackageAdapter(t, "lumi-ark-video.yingce-plugin", "lumi-ark-video")
	tests := []struct {
		name string
		r    GenerationRequest
		fail bool
	}{
		{"audio25", GenerationRequest{Model: "seedance-2.5", Audios: []MediaReference{{URL: "https://example.com/a.mp3"}}}, false},
		{"audio20", GenerationRequest{Model: "seedance-2.0", Audios: []MediaReference{{URL: "https://example.com/a.mp3"}}}, true},
		{"empty", GenerationRequest{Model: "seedance-2.5"}, true},
		{"upgrade", GenerationRequest{Model: "seedance-2.5", ProviderOptions: map[string]map[string]any{"lumi-ark-video": {"draft_task_id": "task_own_sample", "resolution": "1080p"}}}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.r.Extra = map[string]any{"idempotencyKey": "mode-order"}
			s, e := a.BuildCreate(context.Background(), RequestContext{Request: tc.r})
			if (e != nil) != tc.fail {
				t.Fatalf("err=%v", e)
			}
			if tc.name == "upgrade" && e == nil {
				b := manifestTestBody(t, s)
				for _, k := range []string{"duration", "ratio", "generate_audio", "seed"} {
					if _, ok := b[k]; ok {
						t.Errorf("upgrade sends %s", k)
					}
				}
				if len(b["content"].([]any)) != 1 {
					t.Fatal("upgrade content")
				}
			}
		})
	}
}

func TestLumiKeyframeRatioPassthrough(t *testing.T) {
	a := officialPackageAdapter(t, "lumi-ark-video.yingce-plugin", "lumi-ark-video")
	for _, ratio := range []string{"16:9", "adaptive", ""} {
		t.Run("ratio="+ratio, func(t *testing.T) {
			r := GenerationRequest{
				Model: "seedance-2.5", AspectRatio: ratio,
				Images: []MediaReference{
					{URL: "https://example.com/first.png", Role: "first_frame"},
					{URL: "https://example.com/last.png", Role: "last_frame"},
				},
				Extra: map[string]any{"idempotencyKey": "keyframe-order"},
			}
			spec, err := a.BuildCreate(context.Background(), RequestContext{Request: r})
			if err != nil {
				t.Fatal(err)
			}
			body := manifestTestBody(t, spec)
			if got, ok := body["ratio"]; ok != (ratio != "") || (ok && got != ratio) {
				t.Fatalf("ratio = %v (present: %t), want %q", got, ok, ratio)
			}
		})
	}
}

func TestLumiVideoResponses(t *testing.T) {
	a := officialPackageAdapter(t, "lumi-ark-video.yingce-plugin", "lumi-ark-video")
	c := PollContext{TaskID: "task_platform"}
	success := []byte(`{"id":"task_platform","status":"succeeded","duration":5.2,"relay_video_url":"https://example.com/relay.mp4","content":{"video_url":"https://example.com/original.mp4"},"usage":{"total_tokens":10},"billing_status":"settled","consumed_amount":1.25,"currency":"CNY"}`)
	p, e := a.ParsePoll(context.Background(), c, success)
	if e != nil {
		t.Fatal(e)
	}
	if p.Status != StatusSucceeded || p.Result == nil || len(p.Result.Videos) != 1 || p.Result.Videos[0].URL != "https://example.com/relay.mp4" {
		t.Fatalf("result %#v", p)
	}
	for _, raw := range []string{`{"status":"running"}`, `{"status":"succeeded","reconciliation_pending":true,"relay_video_url":"https://example.com/v.mp4"}`, `{"status":"succeeded"}`} {
		p, e = a.ParsePoll(context.Background(), c, []byte(raw))
		if e != nil || p.Status != StatusProcessing {
			t.Fatalf("pending %#v %v", p, e)
		}
	}
	for _, raw := range []string{`{"status":"expired","error":{"code":123,"message":"expired"}}`, `{"status":"failed","error":{"message":"bad"}}`, `{"status":"unknown-future"}`} {
		p, e = a.ParsePoll(context.Background(), c, []byte(raw))
		if e != nil || p.Status != StatusFailed {
			t.Fatalf("failure %#v %v", p, e)
		}
	}
	if _, e = a.ParsePoll(context.Background(), c, []byte("<html>bad</html>")); e == nil {
		t.Fatal("invalid JSON accepted")
	}
}
