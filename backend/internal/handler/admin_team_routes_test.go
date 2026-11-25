package handler

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
	"yingce/backend/internal/auth"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
	"yingce/backend/internal/service"
)

func TestAdminTeamRoutesReadOfficialTeams(t *testing.T) {
	dir := t.TempDir()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: filepath.Join(dir, "admin-teams.db")}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.User{}, &model.AuthSession{}, &model.Team{}, &model.TeamMembership{}); err != nil {
		t.Fatal(err)
	}
	users := []model.User{
		{ID: "site-admin", Username: "site-admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive},
		{ID: "team-owner", Username: "team-owner", DisplayName: "Creator", Role: model.UserRoleUser, Status: model.UserStatusActive},
	}
	teams := []model.Team{
		{ID: "official-company", Name: "公司", OwnerUserID: "team-owner", Status: model.TeamActive},
		{ID: "official-group", Name: "小组", ParentID: "official-company", OwnerUserID: "team-owner", Status: model.TeamActive},
	}
	member := model.TeamMembership{ID: "group-owner", TeamID: "official-group", UserID: "team-owner", Role: model.TeamOwner, Status: model.TeamActive}
	memberships := []model.TeamMembership{
		member,
		{ID: "former-member", TeamID: "official-group", UserID: "team-owner", Role: model.TeamMember, Status: model.TeamLeft},
	}
	sessions := []model.AuthSession{
		{ID: "site-admin-session", UserID: "site-admin", TokenHash: auth.HashToken("fixture-admin-token"), ExpiresAt: time.Now().Add(time.Hour)},
		{ID: "team-owner-session", UserID: "team-owner", TokenHash: auth.HashToken("fixture-owner-token"), ExpiresAt: time.Now().Add(time.Hour)},
	}
	for _, fixture := range []any{&users, &teams, &memberships, &sessions} {
		if err := db.Create(fixture).Error; err != nil {
			t.Fatal(err)
		}
	}

	svc := service.New(repository.New(db), dir)
	t.Cleanup(func() { _ = svc.Close() })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCanvasAPI(router.Group("/api"), svc)
	adminCookie := "site-admin-session.fixture-admin-token"
	ownerCookie := "team-owner-session.fixture-owner-token"
	call := func(path, cookie string, status int, data any) {
		t.Helper()
		rec := profileRequest(router, http.MethodGet, "/api/admin/teams"+path, "", cookie)
		if rec.Code != status {
			t.Fatalf("GET %s = %d, want %d: %s", path, rec.Code, status, rec.Body.String())
		}
		if data != nil {
			var envelope struct {
				Code int             `json:"code"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Code != 0 {
				t.Fatal(rec.Body.String())
			}
			if err := json.Unmarshal(envelope.Data, data); err != nil {
				t.Fatal(err)
			}
		}
	}

	call("", "", http.StatusUnauthorized, nil)
	call("/official-group/members", "", http.StatusUnauthorized, nil)
	call("", ownerCookie, http.StatusForbidden, nil)
	call("/official-group/members", ownerCookie, http.StatusForbidden, nil)

	var list struct {
		Teams []struct {
			ID               string `json:"id"`
			Name             string `json:"name"`
			ParentID         string `json:"parentId"`
			OwnerUserID      string `json:"ownerUserId"`
			Status           string `json:"status"`
			OwnerUsername    string `json:"ownerUsername"`
			OwnerDisplayName string `json:"ownerDisplayName"`
		} `json:"teams"`
	}
	call("", adminCookie, http.StatusOK, &list)
	if len(list.Teams) != 2 {
		t.Fatalf("teams = %+v", list.Teams)
	}
	group := list.Teams[1]
	if group.ID != teams[1].ID || group.Name != teams[1].Name || group.ParentID != teams[0].ID ||
		group.OwnerUserID != member.UserID || group.Status != model.TeamActive ||
		group.OwnerUsername != "team-owner" || group.OwnerDisplayName != "Creator" {
		t.Fatalf("group = %+v", group)
	}
	var roster struct {
		Members []struct {
			ID          string `json:"id"`
			UserID      string `json:"userId"`
			Username    string `json:"username"`
			DisplayName string `json:"displayName"`
			Role        string `json:"role"`
			Status      string `json:"status"`
		} `json:"members"`
	}
	call("/official-group/members", adminCookie, http.StatusOK, &roster)
	if len(roster.Members) != 1 {
		t.Fatalf("members = %+v", roster.Members)
	}
	owner := roster.Members[0]
	if owner.ID != member.ID || owner.UserID != member.UserID || owner.Username != "team-owner" ||
		owner.DisplayName != "Creator" || owner.Role != model.TeamOwner || owner.Status != model.TeamActive {
		t.Fatalf("owner = %+v", owner)
	}
	call("/official-company/members", adminCookie, http.StatusOK, &roster)
	if roster.Members == nil || len(roster.Members) != 0 {
		t.Fatalf("empty team members = %+v", roster.Members)
	}
	call("/unknown/members", adminCookie, http.StatusNotFound, nil)
}
