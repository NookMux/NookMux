package usercontroller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/httpapi/controller/testsupport"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/oauth"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type banAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    BanUserResponse `json:"data"`
}

func createBanTestUser(t *testing.T, id int, mutate func(*userstore.User)) {
	t.Helper()

	u := userstore.User{
		Id:          id,
		Username:    fmt.Sprintf("ban-user-%d", id),
		Password:    "password123",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		DisplayName: fmt.Sprintf("Ban User %d", id),
		Group:       "default",
		AffCode:     fmt.Sprintf("baff%d", id),
	}
	if mutate != nil {
		mutate(&u)
	}
	if err := dbstore.DB.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func requestBan(t *testing.T, role int, body string) (*httptest.ResponseRecorder, banAPIResponse) {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/ban", strings.NewReader(body))
	c.Set("role", role)

	BanUserByIdentifier(c)

	var parsed banAPIResponse
	if err := jsonx.Unmarshal(recorder.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return recorder, parsed
}

func getBanTestUser(t *testing.T, id int) userstore.User {
	t.Helper()

	var u userstore.User
	if err := dbstore.DB.Unscoped().First(&u, id).Error; err != nil {
		t.Fatalf("get user %d: %v", id, err)
	}
	return u
}

func stubGitHubLookup(t *testing.T, fn func(ctx context.Context, login string) (int64, error)) {
	t.Helper()

	old := lookupGitHubLogin
	lookupGitHubLogin = fn
	t.Cleanup(func() { lookupGitHubLogin = old })
}

// expectMsg renders an i18n key with the same machinery the handler uses, so
// assertions hold whether or not i18n has been initialized by an earlier test.
func expectMsg(t *testing.T, key string) string {
	t.Helper()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return httpapi.TranslateMessage(c, key)
}

func TestBanUserByGitHubIdBansExistingUser(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2001, func(u *userstore.User) { u.GitHubId = "583231" })

	recorder, parsed := requestBan(t, common.RoleRootUser, `{"type":"github_id","value":"583231"}`)

	if recorder.Code != http.StatusOK || !parsed.Success {
		t.Fatalf("response = %s, want success", recorder.Body.String())
	}
	if parsed.Data.Result != banResultBannedExisting {
		t.Fatalf("result = %q, want %q", parsed.Data.Result, banResultBannedExisting)
	}
	if got := getBanTestUser(t, 2001).Status; got != common.UserStatusDisabled {
		t.Fatalf("status = %d, want disabled", got)
	}
}

func TestBanUserByGitHubIdAlreadyBanned(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2002, func(u *userstore.User) {
		u.GitHubId = "583231"
		u.Status = common.UserStatusDisabled
	})

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"github_id","value":"583231"}`)

	if !parsed.Success || parsed.Data.Result != banResultAlreadyBanned {
		t.Fatalf("response result = %q, want %q", parsed.Data.Result, banResultAlreadyBanned)
	}
}

func TestBanUserByGitHubIdSoftDeleted(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2003, func(u *userstore.User) {
		u.GitHubId = "583231"
		u.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
	})

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"github_id","value":"583231"}`)

	if !parsed.Success || parsed.Data.Result != banResultAlreadyDeleted {
		t.Fatalf("response result = %q, want %q", parsed.Data.Result, banResultAlreadyDeleted)
	}
	if parsed.Data.User == nil || !parsed.Data.User.Deleted {
		t.Fatalf("response user = %+v, want deleted flag", parsed.Data.User)
	}
}

func TestBanUserByGitHubIdCreatesPlaceholder(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"github_id","value":"583231"}`)

	if !parsed.Success || parsed.Data.Result != banResultCreatedPlaceholder {
		t.Fatalf("response result = %q, want %q", parsed.Data.Result, banResultCreatedPlaceholder)
	}
	u := getBanTestUser(t, parsed.Data.User.Id)
	if u.GitHubId != "583231" {
		t.Fatalf("github_id = %q, want 583231", u.GitHubId)
	}
	if u.Status != common.UserStatusDisabled {
		t.Fatalf("status = %d, want disabled", u.Status)
	}
	if u.Quota != 0 {
		t.Fatalf("quota = %d, want 0 (placeholder must not receive new-user quota)", u.Quota)
	}
	if u.AffCode == "" {
		t.Fatal("aff_code is empty, want generated")
	}
	if !strings.HasPrefix(u.Username, "gh_583231") {
		t.Fatalf("username = %q, want gh_583231 prefix", u.Username)
	}
	var logCount int64
	if err := dbstore.LOG_DB.Model(&logstore.Log{}).Where("user_id = ?", u.Id).Count(&logCount).Error; err != nil {
		t.Fatalf("count logs: %v", err)
	}
	if logCount != 0 {
		t.Fatalf("log count = %d, want 0 (placeholder must not trigger bonus logs)", logCount)
	}
}

func TestBanUserByLinuxDOIdCreatesPlaceholder(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"linuxdo_id","value":"12345"}`)

	if !parsed.Success || parsed.Data.Result != banResultCreatedPlaceholder {
		t.Fatalf("response result = %q, want %q", parsed.Data.Result, banResultCreatedPlaceholder)
	}
	u := getBanTestUser(t, parsed.Data.User.Id)
	if u.LinuxDOId != "12345" {
		t.Fatalf("linux_do_id = %q, want 12345", u.LinuxDOId)
	}
	if !strings.HasPrefix(u.Username, "ldo_12345") {
		t.Fatalf("username = %q, want ldo_12345 prefix", u.Username)
	}
}

func TestBanUserByEmailSingleHit(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2004, func(u *userstore.User) { u.Email = "target@example.com" })

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"email","value":"target@example.com"}`)

	if !parsed.Success || parsed.Data.Result != banResultBannedExisting {
		t.Fatalf("response result = %q, want %q", parsed.Data.Result, banResultBannedExisting)
	}
	if got := getBanTestUser(t, 2004).Status; got != common.UserStatusDisabled {
		t.Fatalf("status = %d, want disabled", got)
	}
}

func TestBanUserByEmailAmbiguousReturnsCandidates(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2005, func(u *userstore.User) { u.Email = "dup@example.com" })
	createBanTestUser(t, 2006, func(u *userstore.User) { u.Email = "dup@example.com" })

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"email","value":"dup@example.com"}`)

	if !parsed.Success || parsed.Data.Result != banResultAmbiguous {
		t.Fatalf("response result = %q, want %q", parsed.Data.Result, banResultAmbiguous)
	}
	if len(parsed.Data.Candidates) != 2 {
		t.Fatalf("candidates = %d, want 2", len(parsed.Data.Candidates))
	}
	for _, id := range []int{2005, 2006} {
		if got := getBanTestUser(t, id).Status; got != common.UserStatusEnabled {
			t.Fatalf("user %d status = %d, ambiguous result must not ban anyone", id, got)
		}
	}
}

func TestBanUserByEmailCreatesPlaceholder(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"email","value":"bad.guy+spam@example.com"}`)

	if !parsed.Success || parsed.Data.Result != banResultCreatedPlaceholder {
		t.Fatalf("response result = %q, want %q", parsed.Data.Result, banResultCreatedPlaceholder)
	}
	u := getBanTestUser(t, parsed.Data.User.Id)
	if u.Email != "bad.guy+spam@example.com" {
		t.Fatalf("email = %q, want placeholder to hold the identifier", u.Email)
	}
	if !strings.HasPrefix(u.Username, "mail_bad_guy_spam") {
		t.Fatalf("username = %q, want sanitized mail_ prefix", u.Username)
	}
}

func TestBanUserRejectsRootTarget(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2007, func(u *userstore.User) {
		u.Role = common.RoleRootUser
		u.Email = "root@example.com"
	})

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"email","value":"root@example.com"}`)

	if parsed.Success {
		t.Fatal("banning root must fail")
	}
	if want := expectMsg(t, i18n.MsgUserCannotDisableRootUser); parsed.Message != want {
		t.Fatalf("message = %q, want %q", parsed.Message, want)
	}
	if got := getBanTestUser(t, 2007).Status; got != common.UserStatusEnabled {
		t.Fatalf("status = %d, root must stay enabled", got)
	}
}

func TestBanUserRejectsHigherRoleWhenAdmin(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2008, func(u *userstore.User) {
		u.Role = common.RoleAdminUser
		u.GitHubId = "777777"
	})

	_, parsed := requestBan(t, common.RoleAdminUser, `{"type":"github_id","value":"777777"}`)

	if parsed.Success {
		t.Fatal("admin banning another admin must fail")
	}
	if want := expectMsg(t, i18n.MsgUserNoPermissionHigherLevel); parsed.Message != want {
		t.Fatalf("message = %q, want %q", parsed.Message, want)
	}
	if got := getBanTestUser(t, 2008).Status; got != common.UserStatusEnabled {
		t.Fatalf("status = %d, target must stay enabled", got)
	}
}

func TestBanUserInvalidIdentifier(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)

	cases := []string{
		`{"type":"github_id","value":"abc"}`,                    // non-numeric GitHub ID
		`{"type":"github_id","value":""}`,                       // empty value
		`{"type":"linuxdo_id","value":"123456789012345678901"}`, // over-length numeric ID
		`{"type":"email","value":"not-an-email"}`,               // malformed email
		`{"type":"email","value":"Name <a@b.com>"}`,             // display-name form is rejected
		`{"type":"github_username","value:"-bad-"}`,             // invalid login charset
		`{"type":"telegram_id","value":"123"}`,                  // unknown type
		`not-json`,                                              // broken body
	}
	for _, body := range cases {
		_, parsed := requestBan(t, common.RoleRootUser, body)
		if parsed.Success {
			t.Fatalf("body %s: want failure, got success", body)
		}
		if want := expectMsg(t, i18n.MsgUserBanInvalidIdentifier); parsed.Message != want {
			t.Fatalf("body %s: message = %q, want %q", body, parsed.Message, want)
		}
	}
}

func TestBanUserByGitHubUsernameFlow(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2009, func(u *userstore.User) { u.GitHubId = "583231" })
	stubGitHubLookup(t, func(ctx context.Context, login string) (int64, error) {
		return 583231, nil
	})

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"github_username","value":"octocat"}`)

	if !parsed.Success || parsed.Data.Result != banResultBannedExisting {
		t.Fatalf("response result = %q, want %q", parsed.Data.Result, banResultBannedExisting)
	}
	if parsed.Data.Resolved == nil || parsed.Data.Resolved.Login != "octocat" || parsed.Data.Resolved.ID != 583231 {
		t.Fatalf("resolved = %+v, want octocat/583231", parsed.Data.Resolved)
	}
	if got := getBanTestUser(t, 2009).Status; got != common.UserStatusDisabled {
		t.Fatalf("status = %d, want disabled", got)
	}
}

func TestBanUserByGitHubUsernameLegacyLoginHit(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	// Pre-migration row: github_id still holds the login string.
	createBanTestUser(t, 2010, func(u *userstore.User) { u.GitHubId = "octocat" })
	stubGitHubLookup(t, func(ctx context.Context, login string) (int64, error) {
		return 583231, nil
	})

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"github_username","value":"octocat"}`)

	if !parsed.Success || parsed.Data.Result != banResultBannedExisting {
		t.Fatalf("response result = %q, want %q (legacy login row must be located)", parsed.Data.Result, banResultBannedExisting)
	}
	if got := getBanTestUser(t, 2010).Status; got != common.UserStatusDisabled {
		t.Fatalf("status = %d, want disabled", got)
	}
}

func TestBanUserByGitHubUsernameLegacyAndNumericAmbiguous(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2011, func(u *userstore.User) { u.GitHubId = "583231" })
	createBanTestUser(t, 2012, func(u *userstore.User) { u.GitHubId = "octocat" })
	stubGitHubLookup(t, func(ctx context.Context, login string) (int64, error) {
		return 583231, nil
	})

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"github_username","value":"octocat"}`)

	if !parsed.Success || parsed.Data.Result != banResultAmbiguous {
		t.Fatalf("response result = %q, want %q", parsed.Data.Result, banResultAmbiguous)
	}
	if len(parsed.Data.Candidates) != 2 {
		t.Fatalf("candidates = %d, want 2", len(parsed.Data.Candidates))
	}
}

func TestBanUserByGitHubUsernameLookupErrors(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)

	cases := []struct {
		name string
		err  error
		key  string
	}{
		{"not found", oauth.ErrGitHubLookupNotFound, i18n.MsgUserBanGitHubUserNotFound},
		{"rate limited", oauth.ErrGitHubLookupRateLimited, i18n.MsgUserBanGitHubRateLimited},
		{"request failed", oauth.ErrGitHubLookupFailed, i18n.MsgUserBanGitHubLookupFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubGitHubLookup(t, func(ctx context.Context, login string) (int64, error) {
				return 0, tc.err
			})
			_, parsed := requestBan(t, common.RoleRootUser, `{"type":"github_username","value":"octocat"}`)
			if parsed.Success {
				t.Fatalf("want failure, got success")
			}
			if want := expectMsg(t, tc.key); parsed.Message != want {
				t.Fatalf("message = %q, want %q", parsed.Message, want)
			}
		})
	}
}

func TestGeneratePlaceholderUsernameCollisionRetry(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2013, func(u *userstore.User) { u.Username = "gh_12345" })

	req := &BanIdentifierRequest{Type: banTypeGitHubId, Value: "12345"}
	username, err := generatePlaceholderUsername(req, nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(username) > placeholderUsernameMaxLen {
		t.Fatalf("username %q exceeds %d chars", username, placeholderUsernameMaxLen)
	}
	if !strings.HasPrefix(username, "gh_12345_") {
		t.Fatalf("username = %q, want gh_12345_ + random suffix", username)
	}
	if userstore.IsUsernameTakenUnscoped(username) {
		t.Fatalf("username %q is still taken", username)
	}
}

func TestBanUserWithRemarkUpdatesExistingUser(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2014, func(u *userstore.User) {
		u.GitHubId = "583231"
		u.Remark = "old note"
	})

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"github_id","value":"583231","remark":" 恶意注册，多次滥用 "}`)

	if !parsed.Success || parsed.Data.Result != banResultBannedExisting {
		t.Fatalf("response = %s, want %q", parsed.Data.Result, banResultBannedExisting)
	}
	if got := getBanTestUser(t, 2014).Remark; got != "恶意注册，多次滥用" {
		t.Fatalf("remark = %q, want trimmed admin note", got)
	}
}

func TestBanUserWithoutRemarkKeepsExistingRemark(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2015, func(u *userstore.User) {
		u.GitHubId = "583231"
		u.Remark = "keep me"
	})

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"github_id","value":"583231"}`)

	if !parsed.Success || parsed.Data.Result != banResultBannedExisting {
		t.Fatalf("response result = %q, want %q", parsed.Data.Result, banResultBannedExisting)
	}
	if got := getBanTestUser(t, 2015).Remark; got != "keep me" {
		t.Fatalf("remark = %q, want preserved", got)
	}
}

func TestBanUserWithRemarkOnPlaceholder(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)

	_, parsed := requestBan(t, common.RoleRootUser, `{"type":"github_id","value":"583231","remark":"spam bot"}`)

	if !parsed.Success || parsed.Data.Result != banResultCreatedPlaceholder {
		t.Fatalf("response result = %q, want %q", parsed.Data.Result, banResultCreatedPlaceholder)
	}
	if got := getBanTestUser(t, parsed.Data.User.Id).Remark; got != "spam bot" {
		t.Fatalf("remark = %q, want admin note", got)
	}
}

func TestBanUserRemarkTooLong(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)

	body := `{"type":"github_id","value":"583231","remark":"` + strings.Repeat("x", 256) + `"}`
	_, parsed := requestBan(t, common.RoleRootUser, body)

	if parsed.Success {
		t.Fatal("over-length remark must fail")
	}
	if want := expectMsg(t, i18n.MsgUserBanRemarkTooLong); parsed.Message != want {
		t.Fatalf("message = %q, want %q", parsed.Message, want)
	}
}

func TestManageUserDisableWithRemark(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2016, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/manage",
		strings.NewReader(`{"id":2016,"action":"disable","remark":"abusive API usage"}`))
	c.Set("role", common.RoleRootUser)
	ManageUser(c)

	if !strings.Contains(recorder.Body.String(), `"success":true`) {
		t.Fatalf("response = %s, want success", recorder.Body.String())
	}
	u := getBanTestUser(t, 2016)
	if u.Status != common.UserStatusDisabled {
		t.Fatalf("status = %d, want disabled", u.Status)
	}
	if u.Remark != "abusive API usage" {
		t.Fatalf("remark = %q, want admin note", u.Remark)
	}
}

func TestManageUserDisableWithoutRemarkKeepsRemark(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	createBanTestUser(t, 2017, func(u *userstore.User) { u.Remark = "keep me" })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/manage",
		strings.NewReader(`{"id":2017,"action":"disable"}`))
	c.Set("role", common.RoleRootUser)
	ManageUser(c)

	if !strings.Contains(recorder.Body.String(), `"success":true`) {
		t.Fatalf("response = %s, want success", recorder.Body.String())
	}
	if got := getBanTestUser(t, 2017).Remark; got != "keep me" {
		t.Fatalf("remark = %q, want preserved", got)
	}
}
