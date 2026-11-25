package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yingce/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func TestRegisterCanvasAPIExposesOpenAPIAndProjects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCanvasAPI(router.Group("/api"), &service.Service{})

	wanted := map[string]bool{
		"GET /api/openapi.yaml":                                             false,
		"GET /api/auth/settings":                                            false,
		"POST /api/auth/login":                                              false,
		"POST /api/auth/register":                                           false,
		"GET /api/admin/settings/turnstile":                                 false,
		"PATCH /api/admin/settings/turnstile":                               false,
		"GET /api/workbench/tools":                                          false,
		"GET /api/workbench/tasks":                                          false,
		"POST /api/workbench/tasks":                                         false,
		"GET /api/projects":                                                 false,
		"GET /api/teams":                                                    false,
		"GET /api/teams/credit-overview":                                    false,
		"POST /api/teams/:teamId/groups":                                    false,
		"POST /api/team-invitations/:id/accept":                             false,
		"GET /api/teams/:teamId/wallet":                                     false,
		"POST /api/teams/:teamId/asset-publications":                        false,
		"POST /api/teams/:teamId/workspace/tasks/quote":                     false,
		"PATCH /api/teams/:teamId/workspace/canvas-projects/:id/operations": false,
		"GET /api/teams/:teamId/workspace/assets":                           false,
		"POST /api/tasks":                                                   false,
		"POST /api/tasks/:id/recover-media":                                 false,
		"GET /api/resources":                                                false,
		"GET /api/skills/presets":                                           false,
		"GET /api/agent/skills/usage":                                       false,
		"GET /api/admin/teams":                                              false,
		"GET /api/admin/teams/:id/members":                                  false,
		"POST /api/wallet/self-adjustment":                                  false,
		"GET /api/fixed-channels":                                           false,
		"PUT /api/fixed-channels/:id/credential":                            false,
		"DELETE /api/fixed-channels/:id/credential":                         false,
	}
	for _, route := range router.Routes() {
		if strings.HasPrefix(route.Path, "/api/company/") || strings.HasPrefix(route.Path, "/api/admin/companies") {
			t.Errorf("legacy company route still registered: %s %s", route.Method, route.Path)
		}
		key := route.Method + " " + route.Path
		if _, exists := wanted[key]; exists {
			wanted[key] = true
		}
	}
	for route, found := range wanted {
		if !found {
			t.Errorf("route %s is not registered", route)
		}
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/openapi.yaml", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "openapi: 3.0.3") || !strings.Contains(recorder.Body.String(), "url: /api") {
		t.Fatalf("openapi.yaml status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, path := range []string{"/admin/teams", "/admin/teams/{id}/members", "/wallet/self-adjustment", "/fixed-channels/{id}/credential", "/teams", "/team-invitations/{id}/accept", "/auth/settings", "/admin/settings/turnstile"} {
		if !strings.Contains(recorder.Body.String(), "\n  "+path+":") {
			t.Errorf("openapi.yaml is missing %s", path)
		}
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/skills/presets", nil))
	if recorder.Code != http.StatusUnauthorized || strings.Contains(recorder.Body.String(), `"presets"`) {
		t.Fatalf("skills presets status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
