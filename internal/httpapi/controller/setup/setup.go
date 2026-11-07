package setupcontroller

import (
	"errors"
	"github.com/NookMux/NookMux/internal/common"
	audit "github.com/NookMux/NookMux/internal/domain/audit"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/db"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/NookMux/NookMux/internal/store/option"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
	"sync"
	"time"
)

// setupMutex 保护安装流程的进程内 check-then-act 竞态：并发安装请求在检查
// shared.Setup / RootUserExists 与实际写入之间可能交错。
// 跨进程/多实例安全由 optionstore.InitializeSetup 的数据库事务 + 固定主键占位保证。
var setupMutex sync.Mutex

type Setup struct {
	Status       bool   `json:"status"`
	RootInit     bool   `json:"root_init"`
	DatabaseType string `json:"database_type"`
}

type SetupRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

func GetSetup(c *gin.Context) {
	setup := Setup{
		Status: shared.Setup,
	}
	if shared.Setup {
		c.JSON(200, gin.H{
			"success": true,
			"data":    setup,
		})
		return
	}
	setup.RootInit = userstore.RootUserExists()
	// UsingOpenGauss/UsingKingbase 时 UsingPostgreSQL 同为 true（PG 兼容内核），统一互斥分支。
	if db.UsingMySQL {
		setup.DatabaseType = "mysql"
	} else if db.UsingOpenGauss {
		setup.DatabaseType = "opengauss"
	} else if db.UsingDM {
		setup.DatabaseType = "dameng"
	} else if db.UsingKingbase {
		setup.DatabaseType = "kingbase"
	} else if db.UsingPostgreSQL {
		setup.DatabaseType = "postgres"
	} else if db.UsingSQLite {
		setup.DatabaseType = "sqlite"
	}
	c.JSON(200, gin.H{
		"success": true,
		"data":    setup,
	})
}

func PostSetup(c *gin.Context) {
	// 安装流程全局只允许一个在途请求：check-then-act（检查 shared.Setup 与
	// RootUserExists 后再写入）没有锁保护时，并发安装请求可能交错创建
	// 重复的 root 用户或 setup 记录。
	setupMutex.Lock()
	defer setupMutex.Unlock()

	// Check if setup is already completed
	if shared.Setup {
		httpapi.ApiErrorI18n(c, i18n.MsgSetupAlreadyInitialized)
		return
	}

	// Check if root user already exists
	rootExists := userstore.RootUserExists()

	var req SetupRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgSetupRequestInvalid)
		return
	}

	// If root doesn't exist, validate and create admin account
	var rootUser *userstore.User
	if !rootExists {
		// Validate username length: max 12 characters to align with userstore.User validation
		if len(req.Username) > 12 {
			httpapi.ApiErrorI18n(c, i18n.MsgSetupUsernameTooLong)
			return
		}
		// Validate password
		if req.Password != req.ConfirmPassword {
			httpapi.ApiErrorI18n(c, i18n.MsgSetupPasswordMismatch)
			return
		}

		if len(req.Password) < 8 {
			httpapi.ApiErrorI18n(c, i18n.MsgSetupPasswordMin)
			return
		}

		// Create root user
		hashedPassword, err := security.Password2Hash(req.Password)
		if err != nil {
			common.SysError("setup: failed to hash password: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgSetupSystemError)
			return
		}
		rootUser = &userstore.User{
			Username:    req.Username,
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: nil,
			Quota:       100000000,
		}
	}

	// 数据库事务 + 固定主键占位：多实例并发初始化时，只有一个实例能成功
	// 写入 setup 记录，其余实例按"已初始化"返回，不会创建重复 root 用户。
	setupRecord := optionstore.NewSetupRecord(common.Version, time.Now().Unix())
	if err := optionstore.InitializeSetup(rootUser, setupRecord); err != nil {
		if errors.Is(err, optionstore.ErrSetupAlreadyInitialized) {
			// 数据库已确认初始化（其他实例率先完成）：同步本进程内存状态，
			// 否则该实例会持续对外报告未初始化直到重启。
			shared.Setup = true
			httpapi.ApiErrorI18n(c, i18n.MsgSetupAlreadyInitialized)
			return
		}
		common.SysError("setup: failed to initialize: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgSetupInitFailed)
		return
	}
	shared.Setup = true

	// 系统初始化时无鉴权，手动设置操作人信息用于审计记录。
	// 审计元数据区分「新建 root 用户」与「复用已有 root 用户」两种初始化路径。
	c.Set("username", req.Username)
	audit.RecordAudit(c, auditstore.AuditModuleSetup, auditstore.AuditActionCreate, "系统初始化", nil, map[string]interface{}{
		"username":          req.Username,
		"root_user_created": !rootExists,
	}, true)

	httpapi.ApiSuccessI18n(c, i18n.MsgSetupInitSuccess, nil)
}
