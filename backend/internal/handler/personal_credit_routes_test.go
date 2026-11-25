package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
	"yingce/backend/internal/auth"
	"yingce/backend/internal/database"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
	"yingce/backend/internal/service"
)

func TestPersonalCreditSelfAdjustment(t *testing.T) {
	const userID = "quota-user"
	const otherID = "other-user"
	const cookie = "quota-session.quota-token"
	const channelID = "quota-fixed-channel"
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Errorf("unexpected Key check request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Header.Get("Authorization") {
		case "Bearer valid-key":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":"example"}]}`))
		case "Bearer rejected-in-body":
			_, _ = w.Write([]byte(`{"success":false,"data":[{"id":"example"}]}`))
		case "Bearer no-models":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "Bearer unexpected-status":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"data":[{"id":"example"}]}`))
		case "Bearer busy":
			w.WriteHeader(http.StatusTooManyRequests)
		case "Bearer unavailable":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	t.Cleanup(upstream.Close)
	dir := t.TempDir()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: filepath.Join(dir, "quota.db")}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{ID: userID, Username: userID, Role: model.UserRoleUser, Status: model.UserStatusActive}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AuthSession{ID: "quota-session", UserID: userID, TokenHash: auth.HashToken("quota-token"), ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.New(db)
	svc := service.New(repo, dir)
	t.Cleanup(func() { _ = svc.Close() })
	if err := svc.EnsureDefaultUserGroup(); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ModelChannel{ID: channelID, Scope: model.ChannelScopeSystem, Enabled: true, CredentialSource: model.ChannelCredentialSourceUser, BaseURL: upstream.URL + "/api/v3"}).Error; err != nil {
		t.Fatal(err)
	}
	user, err := repo.User(userID)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCanvasAPI(router.Group("/api"), svc)
	if err := db.Create(&model.CreditAccount{UserID: otherID, AvailableMicrocredits: 90}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: userID, AvailableMicrocredits: 20, ReservedMicrocredits: 40}).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		session   string
		body      string
		key       string
		status    int
		available int64
	}{
		{"anonymous", "", `"amountMicrocredits":30`, "", http.StatusUnauthorized, 20},
		{"missing Key", cookie, `"amountMicrocredits":30`, "", http.StatusBadRequest, 20},
		{"rejected auth", cookie, `"amountMicrocredits":30`, "bad-key", http.StatusBadRequest, 20},
		{"HTTP 200 with business failure", cookie, `"amountMicrocredits":30`, "rejected-in-body", http.StatusBadRequest, 20},
		{"empty model list", cookie, `"amountMicrocredits":30`, "no-models", http.StatusBadRequest, 20},
		{"unexpected HTTP status", cookie, `"amountMicrocredits":30`, "unexpected-status", http.StatusBadGateway, 20},
		{"upstream rate limit", cookie, `"amountMicrocredits":30`, "busy", http.StatusTooManyRequests, 20},
		{"upstream unavailable", cookie, `"amountMicrocredits":30`, "unavailable", http.StatusBadGateway, 20},
		{"own account", cookie, `"amountMicrocredits":30,"note":"  workspace quota  ","userId":"other-user"`, "valid-key", http.StatusOK, 50},
		{"decrease", cookie, `"amountMicrocredits":-40`, "valid-key", http.StatusOK, 10},
		{"overdraft", cookie, `"amountMicrocredits":-11`, "valid-key", http.StatusBadRequest, 10},
		{"zero", cookie, `"amountMicrocredits":0`, "valid-key", http.StatusBadRequest, 10},
		{"fractional", cookie, `"amountMicrocredits":0.5`, "valid-key", http.StatusBadRequest, 10},
		{"out of range", cookie, `"amountMicrocredits":9007199254740992`, "valid-key", http.StatusBadRequest, 10},
		{"balance overflow", cookie, `"amountMicrocredits":9007199254740991`, "valid-key", http.StatusBadRequest, 10},
		{"balance overflow after releasing reserved credits", cookie, `"amountMicrocredits":9007199254740971`, "valid-key", http.StatusBadRequest, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.key != "" {
				if _, err := svc.SaveFixedChannelCredential(user, channelID, tc.key); err != nil {
					t.Fatal(err)
				}
			}
			body := fmt.Sprintf(`{"channelId":%q,%s}`, channelID, tc.body)
			rec := profileRequest(router, http.MethodPost, "/api/wallet/self-adjustment", body, tc.session)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if tc.status == http.StatusOK {
				var response struct {
					Data model.CreditAccount `json:"data"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.Data.UserID != userID || response.Data.AvailableMicrocredits != tc.available || response.Data.ReservedMicrocredits != 40 {
					t.Fatalf("unexpected account response: %+v", response.Data)
				}
			}
			account, err := repo.CreditAccount(userID)
			if err != nil || account.AvailableMicrocredits != tc.available || account.ReservedMicrocredits != 40 {
				t.Fatalf("account = %+v, err = %v", account, err)
			}
		})
	}
	other, err := repo.CreditAccount(otherID)
	if err != nil || other.AvailableMicrocredits != 90 {
		t.Fatalf("other account = %+v, err = %v", other, err)
	}
	for _, tc := range []struct {
		filter string
		amount int64
		note   string
	}{
		{"income", 30, "workspace quota"},
		{"expenses", -40, ""},
	} {
		items, total, err := repo.WalletHistory(userID, tc.filter, 10, 0)
		if err != nil || total != 1 || len(items) != 1 || items[0].Type != "self_adjustment" || items[0].AmountMicrocredits != tc.amount || items[0].Note != tc.note {
			t.Fatalf("%s history = %+v, total = %d, err = %v", tc.filter, items, total, err)
		}
	}
}
