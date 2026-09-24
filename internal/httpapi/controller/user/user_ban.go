package usercontroller

import (
	"context"
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	audit "github.com/NookMux/NookMux/internal/domain/audit"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/oauth"
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Ban identifier types accepted by POST /api/user/ban.
const (
	banTypeGitHubId       = "github_id"
	banTypeLinuxDOId      = "linuxdo_id"
	banTypeEmail          = "email"
	banTypeGitHubUsername = "github_username"
)

// Outcomes of a ban-by-identifier request.
const (
	banResultBannedExisting     = "banned_existing"
	banResultCreatedPlaceholder = "created_placeholder"
	banResultAlreadyBanned      = "already_banned"
	banResultAlreadyDeleted     = "already_deleted"
	banResultAmbiguous          = "ambiguous"
)

const (
	placeholderUsernameMaxLen   = 20 // must match the User model's username validate limit
	placeholderUsernameAttempts = 5
	githubUsernameLookupTimeout = 15 * time.Second
)

var (
	banNumericIDPattern   = regexp.MustCompile(`^[0-9]{1,20}$`)
	banGitHubLoginPattern = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$`)
)

// lookupGitHubLogin is an indirection over oauth.LookupGitHubLogin so tests can stub it.
var lookupGitHubLogin = oauth.LookupGitHubLogin

type BanIdentifierRequest struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type BanIdentifier struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// ResolvedGitHubUser echoes the result of the GitHub username -> numeric ID lookup.
type ResolvedGitHubUser struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

// BanCandidateDTO describes a located user for both the single-match result and
// the ambiguous candidate list. github_id may hold a legacy login string; it is
// returned as-is so the admin can cross-check.
type BanCandidateDTO struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	GitHubId    string `json:"github_id"`
	LinuxDOId   string `json:"linux_do_id"`
	Role        int    `json:"role"`
	Status      int    `json:"status"`
	Deleted     bool   `json:"deleted"`
}

type BanUserResponse struct {
	Result     string              `json:"result"`
	Identifier BanIdentifier       `json:"identifier"`
	Resolved   *ResolvedGitHubUser `json:"resolved,omitempty"`
	User       *BanCandidateDTO    `json:"user,omitempty"`
	Candidates []BanCandidateDTO   `json:"candidates,omitempty"`
}

// BanUserByIdentifier locates a user by a one-click-login identifier (GitHub /
// LinuxDO numeric ID, email, or GitHub username) and bans them. When no user
// matches, a pre-created banned placeholder is inserted so the identifier is
// rejected on first login; when multiple users match, no one is banned and the
// candidate list is returned for per-item confirmation.
// Only admin users can reach this handler (AdminAuth).
func BanUserByIdentifier(c *gin.Context) {
	var req BanIdentifierRequest
	if err := jsonx.DecodeJson(c.Request.Body, &req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgUserBanInvalidIdentifier)
		return
	}
	req.Value = strings.TrimSpace(req.Value)
	if !validateBanIdentifier(&req) {
		httpapi.ApiErrorI18n(c, i18n.MsgUserBanInvalidIdentifier)
		return
	}

	var resolved *ResolvedGitHubUser
	if req.Type == banTypeGitHubUsername {
		resolved = resolveGitHubUsername(c, req.Value)
		if resolved == nil {
			return // error response already written
		}
	}

	users, err := resolveBanUsers(&req, resolved)
	if err != nil {
		common.SysError("failed to resolve ban candidates: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	resp := BanUserResponse{
		Identifier: BanIdentifier{Type: req.Type, Value: req.Value},
		Resolved:   resolved,
	}
	switch len(users) {
	case 0:
		placeholder := createBannedPlaceholder(c, &req, resolved)
		if placeholder == nil {
			return // error response already written
		}
		resp.Result = banResultCreatedPlaceholder
		resp.User = toBanCandidateDTO(placeholder)
	case 1:
		target := &users[0]
		if target.DeletedAt.Valid {
			// The identifier is already permanently occupied by a deleted user;
			// login with it is rejected, so no action is needed.
			resp.Result = banResultAlreadyDeleted
			resp.User = toBanCandidateDTO(target)
			break
		}
		if target.Status == common.UserStatusDisabled {
			resp.Result = banResultAlreadyBanned
			resp.User = toBanCandidateDTO(target)
			break
		}
		if !banResolvedUser(c, target) {
			return // error response already written
		}
		resp.Result = banResultBannedExisting
		resp.User = toBanCandidateDTO(target)
	default:
		resp.Result = banResultAmbiguous
		resp.Candidates = make([]BanCandidateDTO, 0, len(users))
		for i := range users {
			resp.Candidates = append(resp.Candidates, *toBanCandidateDTO(&users[i]))
		}
	}
	httpapi.ApiSuccess(c, resp)
}

func validateBanIdentifier(req *BanIdentifierRequest) bool {
	switch req.Type {
	case banTypeGitHubId, banTypeLinuxDOId:
		return banNumericIDPattern.MatchString(req.Value)
	case banTypeEmail:
		if len(req.Value) > 50 {
			return false
		}
		addr, err := mail.ParseAddress(req.Value)
		return err == nil && addr.Address == req.Value
	case banTypeGitHubUsername:
		return banGitHubLoginPattern.MatchString(req.Value)
	default:
		return false
	}
}

// resolveGitHubUsername resolves a GitHub login to its numeric ID via the GitHub
// API. It returns nil and writes the error response on any failure.
func resolveGitHubUsername(c *gin.Context, login string) *ResolvedGitHubUser {
	ctx, cancel := context.WithTimeout(c.Request.Context(), githubUsernameLookupTimeout)
	defer cancel()

	id, err := lookupGitHubLogin(ctx, login)
	switch {
	case err == nil:
	case errors.Is(err, oauth.ErrGitHubLookupNotFound):
		httpapi.ApiErrorI18n(c, i18n.MsgUserBanGitHubUserNotFound)
		return nil
	case errors.Is(err, oauth.ErrGitHubLookupRateLimited):
		httpapi.ApiErrorI18n(c, i18n.MsgUserBanGitHubRateLimited)
		return nil
	default:
		common.SysError("github username lookup failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgUserBanGitHubLookupFailed)
		return nil
	}
	return &ResolvedGitHubUser{Login: login, ID: id}
}

// resolveBanUsers returns every user (soft-deleted included) matching the
// identifier, so the caller can distinguish the 0 / 1 / N cases.
func resolveBanUsers(req *BanIdentifierRequest, resolved *ResolvedGitHubUser) ([]userstore.User, error) {
	switch req.Type {
	case banTypeGitHubId:
		return userstore.GetUsersByGitHubIdUnscoped(req.Value)
	case banTypeLinuxDOId:
		return userstore.GetUsersByLinuxDOIdUnscoped(req.Value)
	case banTypeEmail:
		return userstore.GetUsersByEmailUnscoped(req.Value)
	case banTypeGitHubUsername:
		// Probe both the numeric ID and the login string: rows created before
		// the numeric-ID migration still store the login in github_id.
		byID, err := userstore.GetUsersByGitHubIdUnscoped(strconv.FormatInt(resolved.ID, 10))
		if err != nil {
			return nil, err
		}
		byLogin, err := userstore.GetUsersByGitHubIdUnscoped(resolved.Login)
		if err != nil {
			return nil, err
		}
		return mergeUniqueUsers(byID, byLogin), nil
	default:
		return nil, fmt.Errorf("unknown ban identifier type %q", req.Type)
	}
}

func mergeUniqueUsers(groups ...[]userstore.User) []userstore.User {
	seen := make(map[int]struct{})
	merged := make([]userstore.User, 0, 8)
	for _, group := range groups {
		for _, u := range group {
			if _, ok := seen[u.Id]; ok {
				continue
			}
			seen[u.Id] = struct{}{}
			merged = append(merged, u)
		}
	}
	return merged
}

// banResolvedUser disables the located user with the same permission checks as
// ManageUser's "disable" action. It writes the error response and returns false
// on failure.
func banResolvedUser(c *gin.Context, target *userstore.User) bool {
	myRole := c.GetInt("role")
	if myRole <= target.Role && myRole != common.RoleRootUser {
		httpapi.ApiErrorI18n(c, i18n.MsgUserNoPermissionHigherLevel)
		return false
	}
	if target.Role == common.RoleRootUser {
		httpapi.ApiErrorI18n(c, i18n.MsgUserCannotDisableRootUser)
		return false
	}
	before := map[string]any{
		"id": target.Id, "username": target.Username, "status": target.Status,
		"github_id": target.GitHubId, "linux_do_id": target.LinuxDOId, "email": target.Email,
	}
	target.Status = common.UserStatusDisabled
	if err := target.Update(false); err != nil {
		common.SysError("failed to update user during ban: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return false
	}
	invalidateSecuritySensitiveUserCaches(target.Id)
	audit.RecordAudit(c, auditstore.AuditModuleUser, auditstore.AuditActionUpdate,
		"拉黑用户: "+target.Username, before,
		map[string]any{"id": target.Id, "username": target.Username, "status": target.Status})
	return true
}

// createBannedPlaceholder inserts a pre-created banned placeholder user bound to
// the identifier, so the matching account is rejected on its first login. It
// returns nil and writes the error response on failure.
func createBannedPlaceholder(c *gin.Context, req *BanIdentifierRequest, resolved *ResolvedGitHubUser) *userstore.User {
	username, err := generatePlaceholderUsername(req, resolved)
	if err != nil {
		common.SysError("failed to generate placeholder username: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgUserBanPlaceholderFailed)
		return nil
	}

	placeholder := &userstore.User{
		Username:    username,
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusDisabled,
		Remark:      fmt.Sprintf("admin ban placeholder: %s=%s", req.Type, req.Value),
	}
	switch req.Type {
	case banTypeGitHubId:
		placeholder.GitHubId = req.Value
	case banTypeLinuxDOId:
		placeholder.LinuxDOId = req.Value
	case banTypeEmail:
		placeholder.Email = req.Value
	case banTypeGitHubUsername:
		placeholder.GitHubId = strconv.FormatInt(resolved.ID, 10)
	}

	if err := placeholder.InsertBannedPlaceholder(); err != nil {
		common.SysError("failed to insert banned placeholder: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgUserBanPlaceholderFailed)
		return nil
	}
	audit.RecordAudit(c, auditstore.AuditModuleUser, auditstore.AuditActionCreate,
		"拉黑用户（新建占位）: "+placeholder.Username, nil,
		map[string]any{
			"id": placeholder.Id, "username": placeholder.Username, "status": placeholder.Status,
			"type": req.Type, "value": req.Value,
		})
	return placeholder
}

// generatePlaceholderUsername builds a unique username (≤ the model's 20-char
// limit) for a pre-created banned placeholder: gh_<numeric id> / ldo_<id> /
// mail_<email local part>. On collision the base is shortened and a random
// suffix appended, for at most placeholderUsernameAttempts tries.
func generatePlaceholderUsername(req *BanIdentifierRequest, resolved *ResolvedGitHubUser) (string, error) {
	var base string
	switch req.Type {
	case banTypeGitHubId:
		base = "gh_" + req.Value
	case banTypeLinuxDOId:
		base = "ldo_" + req.Value
	case banTypeEmail:
		local, _, _ := strings.Cut(req.Value, "@")
		base = "mail_" + sanitizePlaceholderName(local)
	case banTypeGitHubUsername:
		base = "gh_" + strconv.FormatInt(resolved.ID, 10)
	}
	base = truncateASCII(base, placeholderUsernameMaxLen)

	for attempt := range placeholderUsernameAttempts {
		candidate := base
		if attempt > 0 {
			suffix := "_" + common.GetRandomString(2)
			candidate = truncateASCII(base, placeholderUsernameMaxLen-len(suffix)) + suffix
		}
		if !userstore.IsUsernameTakenUnscoped(candidate) {
			return candidate, nil
		}
	}
	return "", errors.New("placeholder username collision retry exhausted")
}

// sanitizePlaceholderName replaces characters outside [a-zA-Z0-9_] with "_" and
// falls back to "user" when nothing remains.
func sanitizePlaceholderName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "user"
	}
	return out
}

func truncateASCII(s string, maxLen int) string {
	if maxLen < 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func toBanCandidateDTO(u *userstore.User) *BanCandidateDTO {
	return &BanCandidateDTO{
		Id:          u.Id,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		GitHubId:    u.GitHubId,
		LinuxDOId:   u.LinuxDOId,
		Role:        u.Role,
		Status:      u.Status,
		Deleted:     u.DeletedAt.Valid,
	}
}
