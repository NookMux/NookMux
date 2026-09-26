package middleware

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func setupAuthAccessTokenTestDB(t *testing.T) {
	t.Helper()

	if err := i18n.Init(); err != nil {
		t.Fatalf("init i18n: %v", err)
	}

	oldDB := dbstore.DB
	oldRedisEnabled := redis.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := db.AutoMigrate(&userstore.User{}, &tokenstore.Token{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	dbstore.DB = db
	redis.RedisEnabled = false
	common.MemoryCacheEnabled = false

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		redis.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
}

func TestUserAuthRejectsEmptyBearerMatchingEmptyAccessToken(t *testing.T) {
	setupAuthAccessTokenTestDB(t)
	gin.SetMode(gin.TestMode)

	emptyAccessToken := ""
	rootUser := userstore.User{
		Id:          1,
		Username:    "root",
		Password:    "password123",
		Role:        common.RoleRootUser,
		Status:      common.UserStatusEnabled,
		DisplayName: "Root User",
		AccessToken: &emptyAccessToken,
		Group:       "default",
		AffCode:     "auth-aff-root-empty",
	}
	if err := dbstore.DB.Create(&rootUser).Error; err != nil {
		t.Fatalf("create root user: %v", err)
	}

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.GET("/protected", UserAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"id":      c.GetInt("id"),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer ")
	req.Header.Set("New-Api-User", "1")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if strings.Contains(recorder.Body.String(), `"id":1`) {
		t.Fatalf("empty bearer authenticated root user, response: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "access token 无效") {
		t.Fatalf("expected invalid access token response, got: %s", recorder.Body.String())
	}
}

func TestUserAuthDoesNotUseCommonUserEmptyAccessTokenAsRoot(t *testing.T) {
	setupAuthAccessTokenTestDB(t)
	gin.SetMode(gin.TestMode)

	rootUser := userstore.User{
		Id:          1,
		Username:    "root",
		Password:    "password123",
		Role:        common.RoleRootUser,
		Status:      common.UserStatusEnabled,
		DisplayName: "Root User",
		Group:       "default",
		AffCode:     "auth-aff-root",
	}
	emptyAccessToken := ""
	commonUser := userstore.User{
		Id:          2,
		Username:    "common",
		Password:    "password123",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		DisplayName: "Common User",
		AccessToken: &emptyAccessToken,
		Group:       "default",
		AffCode:     "auth-aff-common",
	}
	if err := dbstore.DB.Create(&rootUser).Error; err != nil {
		t.Fatalf("create root user: %v", err)
	}
	if err := dbstore.DB.Create(&commonUser).Error; err != nil {
		t.Fatalf("create common user: %v", err)
	}

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.GET("/protected", UserAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"id":      c.GetInt("id"),
			"role":    c.GetInt("role"),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer ")
	req.Header.Set("New-Api-User", "1")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if strings.Contains(recorder.Body.String(), `"id":1`) || strings.Contains(recorder.Body.String(), `"role":100`) {
		t.Fatalf("common user empty access token authenticated as root, response: %s", recorder.Body.String())
	}
}

func TestUserAuthHidesAccessTokenDatabaseErrors(t *testing.T) {
	setupAuthAccessTokenTestDB(t)
	gin.SetMode(gin.TestMode)

	accessToken := "management-token-db-error"
	user := userstore.User{
		Id:          1,
		Username:    "root",
		Password:    "password123",
		Role:        common.RoleRootUser,
		Status:      common.UserStatusEnabled,
		DisplayName: "Root User",
		AccessToken: &accessToken,
		Group:       "default",
		AffCode:     "auth-aff-root-db-error",
	}
	if err := dbstore.DB.Create(&user).Error; err != nil {
		t.Fatalf("create root user: %v", err)
	}
	sqlDB, err := dbstore.DB.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sql db: %v", err)
	}

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.GET("/protected", UserAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("New-Api-User", "1")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "access token 无效") {
		t.Fatalf("expected generic invalid access token response, got: %s", body)
	}
	if strings.Contains(body, "database") || strings.Contains(body, "数据库错误") || strings.Contains(body, "sql: database is closed") {
		t.Fatalf("response leaked backend database details: %s", body)
	}
}

func TestTokenAuthHidesUserCacheErrors(t *testing.T) {
	setupAuthAccessTokenTestDB(t)
	gin.SetMode(gin.TestMode)

	token := tokenstore.Token{
		UserId:         404,
		Key:            "tokenauthhiddenusererror",
		Name:           "hidden-error",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
	}
	if err := dbstore.DB.Create(&token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	router := gin.New()
	router.GET("/relay", TokenAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/relay", nil)
	req.Header.Set("Authorization", "Bearer "+token.Key)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "数据库错误，请稍后重试") {
		t.Fatalf("expected generic database error, got: %s", body)
	}
	if strings.Contains(body, "record not found") || strings.Contains(body, "404") {
		t.Fatalf("response leaked backend lookup details: %s", body)
	}
}

func TestTokenAuthReadOnlyHidesUserCacheErrors(t *testing.T) {
	setupAuthAccessTokenTestDB(t)
	gin.SetMode(gin.TestMode)

	token := tokenstore.Token{
		UserId:         405,
		Key:            "readonlyhiddenusererror",
		Name:           "readonly-hidden-error",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
	}
	if err := dbstore.DB.Create(&token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	router := gin.New()
	router.GET("/readonly", TokenAuthReadOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/readonly", nil)
	req.Header.Set("Authorization", "Bearer "+token.Key)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "数据库错误，请稍后重试") {
		t.Fatalf("expected generic database error, got: %s", body)
	}
	if strings.Contains(body, "record not found") || strings.Contains(body, "405") {
		t.Fatalf("response leaked backend lookup details: %s", body)
	}
}

func TestSecureVerificationRejectsTimestampWithoutMatchingUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.GET("/protected",
		func(c *gin.Context) {
			c.Set("id", 1)
			session := sessions.Default(c)
			session.Set(SecureVerificationSessionKey, time.Now().Unix())
			if err := session.Save(); err != nil {
				t.Fatalf("save session: %v", err)
			}
			c.Next()
		},
		SecureVerificationRequired(),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"success": true})
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	body := recorder.Body.String()
	if strings.Contains(body, `"success":true`) {
		t.Fatalf("secure verification accepted timestamp without user binding: %s", body)
	}
	if !strings.Contains(body, "VERIFICATION_INVALID") {
		t.Fatalf("expected invalid verification response, got: %s", body)
	}
}
