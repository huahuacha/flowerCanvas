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

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

func TestFixedChannelCredentialsBelongToTheAuthenticatedMember(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: filepath.Join(t.TempDir(), "test.db")}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	if err := db.AutoMigrate(&model.Company{}, &model.CompanyMember{}, &model.User{}, &model.UserGroup{}, &model.UserGroupChannel{}, &model.ModelChannel{}, &model.UserChannelCredential{}, &model.RouteAttempt{}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db), dataDir: t.TempDir()}
	if err := os.WriteFile(filepath.Join(svc.dataDir, ".settings-key"), make([]byte, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureDefaultUserGroup(); err != nil {
		t.Fatal(err)
	}
	owner := model.User{ID: "owner", Username: "owner", Role: model.UserRoleUser, Status: model.UserStatusActive}
	other := model.User{ID: "other", Username: "other", Role: model.UserRoleUser, Status: model.UserStatusActive}
	company := model.Company{ID: "company", Name: "Company"}
	channel := model.ModelChannel{ID: "fixed", Scope: model.ChannelScopeSystem, Enabled: true, CredentialSource: model.ChannelCredentialSourceUser}
	for _, fixture := range []any{&owner, &other, &company, &channel, &model.CompanyMember{ID: "member", UserID: owner.ID, CompanyID: company.ID, Role: model.CompanyMemberRoleAdmin}} {
		if err := db.Create(fixture).Error; err != nil {
			t.Fatal(err)
		}
	}
	const key = "member-credential"
	if _, err := svc.fixedChannelAPIKey(owner.ID, &channel); err == nil {
		t.Fatal("eligible member can submit without a credential")
	} else {
		var appErr *AuthError
		if !errors.As(err, &appErr) || appErr.Reason != "FIXED_CHANNEL_CREDENTIAL_REQUIRED" {
			t.Fatalf("missing credential error = %v", err)
		}
	}
	if _, err := svc.SaveFixedChannelCredential(&other, channel.ID, key); err == nil {
		t.Fatal("non-member can configure a credential")
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
	if _, err := svc.fixedChannelAPIKey(other.ID, &channel); err == nil {
		t.Fatal("non-member can resolve a credential")
	}

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
