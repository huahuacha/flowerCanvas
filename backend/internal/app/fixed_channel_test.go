package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
	"yingce/backend/internal/model"
	"yingce/backend/internal/protocol"
	"yingce/backend/internal/repository"
)

func TestFixedChannelCredentialsBelongToTheAuthorizedUser(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: filepath.Join(t.TempDir(), "test.db")}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	if err := db.AutoMigrate(&model.User{}, &model.UserGroup{}, &model.UserGroupChannel{}, &model.ModelChannel{}, &model.UserChannelCredential{}, &model.RouteAttempt{}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db), dataDir: t.TempDir(), routeHealthBlocked: make(map[string]time.Time)}
	if err := os.WriteFile(filepath.Join(svc.dataDir, ".settings-key"), make([]byte, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureDefaultUserGroup(); err != nil {
		t.Fatal(err)
	}
	owner := model.User{ID: "owner", Username: "owner", Role: model.UserRoleUser, Status: model.UserStatusActive}
	other := model.User{ID: "other", Username: "other", Role: model.UserRoleUser, Status: model.UserStatusActive, UserGroup: "restricted"}
	channel := model.ModelChannel{ID: "fixed", Scope: model.ChannelScopeSystem, Enabled: true, CredentialSource: model.ChannelCredentialSourceUser}
	for _, fixture := range []any{&owner, &other, &channel, &model.UserGroup{ID: "restricted", Code: "restricted", Name: "Restricted"}} {
		if err := db.Create(fixture).Error; err != nil {
			t.Fatal(err)
		}
	}
	const key = "member-credential"
	if list, err := svc.FixedChannels(&owner); err != nil || len(list.Channels) != 1 || list.Channels[0].HasCredential {
		t.Fatalf("authorized user without a company can see the channel and its missing credential: %v, %v", list, err)
	}
	if list, err := svc.FixedChannels(&other); err != nil || len(list.Channels) != 0 {
		t.Fatalf("restricted group can see the fixed channel: %v, %v", list, err)
	}
	input := map[string]any{"config": map[string]any{"channelId": channel.ID}}
	if err := svc.requireFixedTaskCredential(owner.ID, input); err == nil {
		t.Fatal("authorized user can submit without a credential")
	} else {
		var appErr *AuthError
		if !errors.As(err, &appErr) || appErr.Reason != "FIXED_CHANNEL_CREDENTIAL_REQUIRED" {
			t.Fatalf("missing credential error = %v", err)
		}
	}
	if _, err := svc.SaveFixedChannelCredential(&other, channel.ID, key); err == nil {
		t.Fatal("restricted group can configure a credential")
	}
	view, err := svc.SaveFixedChannelCredential(&owner, channel.ID, key)
	if err != nil || !view.HasCredential {
		t.Fatalf("saving own credential: %v", err)
	}
	encoded, err := json.Marshal(view)
	if err != nil || strings.Contains(string(encoded), key) {
		t.Fatal("credential view contains the key")
	}
	var row model.UserChannelCredential
	if err := db.First(&row, "user_id = ? AND channel_id = ?", owner.ID, channel.ID).Error; err != nil || row.APIKeyCipher == "" || strings.Contains(row.APIKeyCipher, key) {
		t.Fatalf("stored credential must be encrypted: %v", err)
	}
	if got, err := svc.fixedChannelAPIKey(owner.ID, &channel); err != nil || got != key {
		t.Fatalf("owner credential unavailable: %v", err)
	}
	if err := svc.requireFixedTaskCredential(owner.ID, input); err != nil {
		t.Fatalf("task submission rejected the configured credential: %v", err)
	}
	if _, err := svc.fixedChannelAPIKey(other.ID, &channel); err == nil {
		t.Fatal("restricted group can resolve a credential")
	}

	t.Run("fixed route keeps its own credentials on failure", func(t *testing.T) {
		task := model.Task{UserID: owner.ID, LogicalModelID: "logical", InputJSON: `{"config":{"channelId":"fixed"}}`}
		if _, err := svc.switchTaskToNextRoute(&task, nil); err == nil || !strings.Contains(err.Error(), "固定渠道任务不能切换渠道") {
			t.Fatalf("expected the fixed channel to stop failover before loading other routes: %v", err)
		}
		attempt := model.RouteAttempt{ChannelID: channel.ID, DispatchState: "rejected_no_job", FailureCode: "upstream_401"}
		next, err := svc.nextRouteAttemptAfterFailure(&task, &attempt, errors.New("upstream rejected personal key"))
		if next != nil || err != nil || len(svc.routeHealthBlocked) != 0 {
			t.Fatalf("personal key failure started failover or blocked the shared route: %v, %v, %v", next, err, svc.routeHealthBlocked)
		}
	})

	t.Run("manual retry checks the original channel credential before routing", func(t *testing.T) {
		task := model.Task{UserID: other.ID, LogicalModelID: "logical"}
		if err := db.Model(&other).Update("user_group", model.DefaultUserGroupCode).Error; err != nil {
			t.Fatal(err)
		}
		err := svc.prepareLogicalTaskRetry(&task, input)
		var appErr *AuthError
		if !errors.As(err, &appErr) || appErr.Reason != "FIXED_CHANNEL_CREDENTIAL_REQUIRED" {
			t.Fatalf("manual retry should require a personal key on the original channel: %v", err)
		}
	})

	t.Run("submitted task retains the original credential", func(t *testing.T) {
		task := model.Task{ID: "task", UserID: owner.ID, RouteRun: 1, ProviderRequestID: "upstream-task"}
		attempt := model.RouteAttempt{ID: "attempt", TaskID: task.ID, RouteRun: task.RouteRun, AttemptNumber: 1, DispatchState: "submission_unknown"}
		if err := svc.repo.CreateRouteAttempt(&attempt); err != nil {
			t.Fatal(err)
		}
		ctx := withTaskExecutionID(withProviderSubmissionKey(context.Background(), &attempt), task.ID)
		original := providerConfig{
			ChannelID: channel.ID, BaseURL: "https://8.8.8.8/v1", APIFormat: "openai", InterfaceType: "lumi-ark-video",
			APIKey: key, Model: "seedance", credentialSource: model.ChannelCredentialSourceUser,
		}
		if err := svc.saveTaskSubmissionCredential(ctx, owner.ID, original); err != nil {
			t.Fatal(err)
		}
		attempt.DispatchState = "accepted"
		if err := svc.repo.SaveRouteAttempt(&attempt); err != nil {
			t.Fatal(err)
		}
		if err := svc.DeleteFixedChannelCredential(&owner, channel.ID); err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&owner).Update("status", model.UserStatusDisabled).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&channel).Update("enabled", false).Error; err != nil {
			t.Fatal(err)
		}
		resumed, err := svc.resolveSubmittedTaskProviderConfig(&task, providerConfig{ChannelID: "initial-route"})
		if err != nil || resumed.APIKey != original.APIKey || resumed.BaseURL != original.BaseURL || resumed.InterfaceType != original.InterfaceType {
			t.Fatalf("submitted task must use its recorded channel, endpoint, protocol and key: %v", err)
		}
		svc.pluginRuntime, err = newPluginRuntime(svc.dataDir)
		if err != nil {
			t.Fatal(err)
		}
		releases := svc.protocolRegistry().Releases(original.InterfaceType)
		if len(releases) != 1 {
			t.Fatalf("expected one bundled Lumi release, got %d", len(releases))
		}
		binding, err := json.Marshal(taskProtocolBinding{
			Digest: releases[0].SHA256, ProviderID: original.InterfaceType,
			ChannelID: channel.ID, ChannelModelID: "fixed-model", UpstreamModel: original.Model,
		})
		if err != nil {
			t.Fatal(err)
		}
		task.ProtocolBindingJSON = string(binding)
		if err := db.Model(&attempt).Updates(map[string]any{"protocol_binding_json": task.ProtocolBindingJSON, "channel_model_id": "fixed-model"}).Error; err != nil {
			t.Fatal(err)
		}
		pinned, err := svc.resolveSubmittedTaskProviderConfig(&task, providerConfig{ChannelID: "initial-route"})
		if err != nil || !pinned.pinned || pinned.registry != releases[0].Registry || pinned.APIKey != original.APIKey {
			t.Fatalf("resumed fixed task lost the pinned plugin or credential: %v", err)
		}
		task.RouteRun++
		if _, err := svc.resolveSubmittedTaskProviderConfig(&task, providerConfig{ChannelID: channel.ID}); err == nil {
			t.Fatal("manual retry reused the previous submission credential")
		}
	})
}

func TestFixedChannelRedactsKeyAcrossResponseChunks(t *testing.T) {
	redactor := fixedProviderStreamRedactor{credential: []byte("member-credential")}
	var result strings.Builder
	for _, chunk := range []string{"error: mem", "ber-cred", "ential; member-credential; member"} {
		result.Write(redactor.consume([]byte(chunk), false))
	}
	result.Write(redactor.consume(nil, true))
	if result.String() != "error: [REDACTED]; [REDACTED]; member" {
		t.Fatalf("unexpected redacted response %q", result.String())
	}
}

func TestFixedChannelMediaDownloadRejectsCredentialRedirect(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	if err := db.AutoMigrate(&model.SystemSetting{}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db), dataDir: t.TempDir()}
	const key = "member-credential"
	var destinationHits atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinationHits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destination.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != key {
			t.Error("configured origin did not receive its credential")
		}
		http.Redirect(w, r, destination.URL+"/result", http.StatusFound)
	}))
	defer upstream.Close()

	config := providerConfig{BaseURL: upstream.URL, APIFormat: "gemini", APIKey: key, credentialSource: model.ChannelCredentialSourceUser}
	_, _, err = svc.downloadTaskMedia(context.Background(), config, upstream.URL+"/result", "image")
	if err == nil || !strings.Contains(err.Error(), "不能重定向到其他上游地址") || destinationHits.Load() != 0 {
		t.Fatalf("expected redirect rejection before reaching destination, got err=%v hits=%d", err, destinationHits.Load())
	}
}

func TestFixedChannelPreservesUpstreamResponseForParsing(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	const body = `{"status":"succeeded","id":"task-status"}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer upstream.Close()
	config := providerConfig{BaseURL: upstream.URL, APIKey: "status", credentialSource: model.ChannelCredentialSourceUser}
	ctx := withFixedProviderCredential(context.Background(), config)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	applyProviderAuth(req, config)
	data, _, err := doBinary(req)
	if err != nil || string(data) != body {
		t.Fatalf("protocol parser must receive unmodified response: %q, %v", data, err)
	}
}

func TestFixedChannelLogsPreserveAsyncProtocolReceipt(t *testing.T) {
	svc, db := generationLogService(t)
	adapter, ok := svc.protocolRegistry().Resolve("lumi-ark-video")
	if !ok {
		t.Fatal("Lumi adapter unavailable")
	}
	task := model.Task{ID: "fixed-video", UserID: "owner", Type: "canvas_video", Status: model.TaskStatusRunning}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	config := providerConfig{BaseURL: "https://upstream.invalid", APIKey: "id", credentialSource: model.ChannelCredentialSourceUser}
	ctx := withFixedProviderCredential(withProviderAnalytics(context.Background(), svc, task), config)
	receipt := protocolHTTPReceipt{parse: func(body []byte) (protocol.CreateResult, error) { return adapter.ParseCreate(ctx, body) }}
	ctx = withProtocolHTTPReceipt(withProviderRequestKind(ctx, "create"), &receipt)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.BaseURL+"/api/v3/contents/generations/tasks", nil)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"id":"upstream-video-id-1","status":"queued","message":"credential id"}`)
	recordProviderRequest(req, time.Now(), http.StatusOK, body, nil)
	result, err := receipt.decode(ctx, body)
	if err != nil || result.TaskID != "upstream-video-id-1" || result.Status != protocol.StatusProcessing {
		t.Fatalf("receipt = %#v, err = %v", result, err)
	}
	var log model.ApiCallLog
	if err := db.First(&log, "task_id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(log.ResponseBody, "credential id") || log.ProviderRequestID != result.TaskID {
		t.Fatalf("log lost the provider ID or retained the credential: %#v", log)
	}

	const echoedKey = `fixed-test-credential-"quoted"`
	ctx = withFixedProviderCredential(ctx, providerConfig{BaseURL: config.BaseURL, APIKey: echoedKey, credentialSource: model.ChannelCredentialSourceUser})
	receipt = protocolHTTPReceipt{parse: func(body []byte) (protocol.CreateResult, error) { return adapter.ParseCreate(ctx, body) }}
	body = []byte(`{"id":"job-fixed-test-credential-\"quoted\"","status":"queued","message":"fixed-test-credential-\"quoted\""}`)
	recordProviderRequest(req.WithContext(ctx), time.Now(), http.StatusOK, body, nil)
	if result, err := receipt.decode(ctx, body); err == nil || result.TaskID != "" || strings.Contains(err.Error(), echoedKey) {
		t.Fatalf("echoed credential was accepted as a task ID: %#v, %v", result, err)
	}
	var rejected model.ApiCallLog
	if err := db.First(&rejected, "task_id = ? AND id <> ?", task.ID, log.ID).Error; err != nil {
		t.Fatal(err)
	}
	if rejected.ProviderRequestID != "" || strings.Contains(rejected.ResponseBody, "fixed-test-credential-") {
		t.Fatalf("echoed credential persisted in the call log: %#v", rejected)
	}
}
