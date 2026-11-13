package protocol

import (
	"context"
	"testing"
)

func TestLumiImageNewAPIContract(t *testing.T) {
	ctx := context.Background()
	a := officialPackageAdapter(t, "lumi-image.yingce-plugin", "lumi-image")
	request := GenerationRequest{
		Capability: CapabilityImage, Model: "seedream5.5", Prompt: "a cat", AspectRatio: "16:9",
		Extra:  map[string]any{"idempotencyKey": "image-order"},
		Images: []MediaReference{{URL: "https://example.com/input.png"}},
		ImageParameters: map[string]any{
			"seed": 0, "watermark": false, "optimize_prompt": true,
		},
	}
	spec, err := a.BuildCreate(ctx, RequestContext{BaseURL: "https://example.com", Request: request})
	if err != nil {
		t.Fatal(err)
	}
	body := manifestTestBody(t, spec)
	if spec.Path != "/v1/images/jobs" || body["idempotency_key"] != "image-order" || body["user_prompt"] != "a cat" {
		t.Fatalf("unexpected create request: %s %#v", spec.Path, body)
	}
	params := body["parameters"].(map[string]any)
	if params["size"] != "2560x1440" || params["seed"] != float64(0) || params["watermark"] != false ||
		params["optimize_prompt"] != true || params["optimize_prompt_options"] != nil {
		t.Fatalf("unexpected Seedream 5.5 parameters: %#v", params)
	}
	refs := body["reference_images"].([]any)
	if len(refs) != 1 || refs[0] != "https://example.com/input.png" {
		t.Fatalf("unexpected references: %#v", refs)
	}
	created, err := a.ParseCreate(ctx, []byte(`{"job_id":"task_image","status":"queued"}`))
	if err != nil || created.TaskID != "task_image" || created.Status != StatusProcessing {
		t.Fatalf("create response: %#v, %v", created, err)
	}
	poll := PollContext{TaskID: created.TaskID, BaseURL: "https://example.com"}
	pollSpec, err := a.BuildPoll(ctx, poll)
	if err != nil || pollSpec.Path != "/v1/images/jobs/task_image" {
		t.Fatalf("poll request: %#v, %v", pollSpec, err)
	}
	result, err := a.ParsePoll(ctx, poll, []byte(`{"status":"succeeded","results":[{"stable_url":"https://example.com/private","tos_url":"https://example.com/signed"}]}`))
	if err != nil || result.Status != StatusSucceeded || result.Result == nil || len(result.Result.Images) != 1 ||
		result.Result.Images[0].URL != "https://example.com/signed" || !result.Result.Images[0].Ephemeral {
		t.Fatalf("poll response: %#v, %v", result, err)
	}
	for _, status := range []string{"submitting", "polling", "transferring"} {
		result, err := a.ParsePoll(ctx, poll, []byte(`{"status":"`+status+`"}`))
		if err != nil || result.Status != StatusProcessing {
			t.Fatalf("%s: %#v, %v", status, result, err)
		}
	}
	failed, err := a.ParsePoll(ctx, poll, []byte(`{"status":"failed","error":{"code":"provider_algorithm_failed","message":"Image generation failed"}}`))
	if err == nil && failed.Status != StatusFailed {
		t.Fatalf("failed task response: %#v", failed)
	}
}
