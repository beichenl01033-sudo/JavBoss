package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	dbpkg "javboss/internal/db"
)

func TestPlayerPerformanceSettingsValidationAndPersistence(t *testing.T) {
	deps := newTestDependencies()
	testAuthService(deps, t)
	router := gin.New()
	router.PATCH("/config", updateConfig)
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"player_seek_mode":"adaptive","player_hwdec":"no"}`, 200},
		{`{"player_seek_mode":"exact","player_hwdec":"auto"}`, 200},
		{`{"player_seek_mode":"bad"}`, 400},
		{`{"player_hwdec":"auto-unsafe"}`, 400},
		{`{"player_hwdec":""}`, 400},
		{`{"player_seek_mode":"adaptive","player_hwdec":"bad"}`, 400},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("PATCH", "/config", strings.NewReader(tc.body)))
		if w.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.body, w.Code, w.Body)
		}
	}
	config, err := dbpkg.Default().ListConfig(context.Background())
	if err != nil || config["player_seek_mode"] != "exact" || config["player_hwdec"] != "auto" {
		t.Fatalf("config %v: %v", config, err)
	}
}
