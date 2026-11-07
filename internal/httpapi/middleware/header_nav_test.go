package middleware

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withHeaderNavModules(t *testing.T, raw string) {
	t.Helper()

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	previous, hadPrevious := common.OptionMap["HeaderNavModules"]
	common.OptionMap["HeaderNavModules"] = raw
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if hadPrevious {
			common.OptionMap["HeaderNavModules"] = previous
			return
		}
		delete(common.OptionMap, "HeaderNavModules")
	})
}

// setupHeaderNavTestDB initialises an in-memory SQLite DB with a single enabled
// common user (id=1) so the session re-validation path in authHelper can look
// up the latest role/status.
func setupHeaderNavTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	oldRedisEnabled := redis.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := db.AutoMigrate(&userstore.User{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	dbstore.DB = db
	redis.RedisEnabled = false
	common.MemoryCacheEnabled = false

	if err := dbstore.DB.Create(&userstore.User{
		Id:       1,
		Username: "tester",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "header-nav-aff",
	}).Error; err != nil {
		t.Fatalf("create header nav test user: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		redis.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
}

func performHeaderNavRequest(t *testing.T, handler gin.HandlerFunc, authenticated bool) *httptest.ResponseRecorder {
	t.Helper()

	if authenticated {
		setupHeaderNavTestDB(t)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("header-nav-test"))))
	router.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "tester")
		session.Set("role", common.RoleCommonUser)
		session.Set("id", 1)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		// 与 SetupLogin 保持一致：登录时快照凭证版本（建表默认值为 1）。
		session.Set("auth_version", 1)
		if err := session.Save(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false})
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/api/test", handler, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	var cookies []*http.Cookie
	if authenticated {
		loginRecorder := httptest.NewRecorder()
		loginRequest := httptest.NewRequest(http.MethodGet, "/login", nil)
		router.ServeHTTP(loginRecorder, loginRequest)
		if loginRecorder.Code != http.StatusNoContent {
			t.Fatalf("login status = %d, body = %s", loginRecorder.Code, loginRecorder.Body.String())
		}
		cookies = loginRecorder.Result().Cookies()
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	if authenticated {
		request.Header.Set("New-Api-User", "1")
		for _, cookie := range cookies {
			request.AddCookie(cookie)
		}
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestHeaderNavModuleAuthAllowsDefaultPublicAccess(t *testing.T) {
	withHeaderNavModules(t, "")

	recorder := performHeaderNavRequest(t, HeaderNavModuleAuth("pricing"), false)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestHeaderNavModuleAuthRejectsDisabledModule(t *testing.T) {
	withHeaderNavModules(t, `{"pricing":{"enabled":false,"requireAuth":false}}`)

	recorder := performHeaderNavRequest(t, HeaderNavModuleAuth("pricing"), false)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestHeaderNavModuleAuthRequiresLogin(t *testing.T) {
	withHeaderNavModules(t, `{"pricing":{"enabled":true,"requireAuth":true}}`)

	recorder := performHeaderNavRequest(t, HeaderNavModuleAuth("pricing"), false)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestHeaderNavModuleAuthAllowsLoggedInUser(t *testing.T) {
	withHeaderNavModules(t, `{"pricing":{"enabled":true,"requireAuth":true}}`)

	recorder := performHeaderNavRequest(t, HeaderNavModuleAuth("pricing"), true)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestHeaderNavModuleAuthRejectsLegacyDisabledModule(t *testing.T) {
	withHeaderNavModules(t, `{"pricing":false}`)

	recorder := performHeaderNavRequest(t, HeaderNavModuleAuth("pricing"), false)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", recorder.Code, recorder.Body.String())
	}
}
