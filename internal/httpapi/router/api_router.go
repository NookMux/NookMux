package router

import (
	"github.com/NookMux/NookMux/internal/httpapi/controller/audit"
	"github.com/NookMux/NookMux/internal/httpapi/controller/billing"
	"github.com/NookMux/NookMux/internal/httpapi/controller/channel"
	"github.com/NookMux/NookMux/internal/httpapi/controller/checkin"
	"github.com/NookMux/NookMux/internal/httpapi/controller/custom_voice"
	"github.com/NookMux/NookMux/internal/httpapi/controller/db_migrate"
	"github.com/NookMux/NookMux/internal/httpapi/controller/dynamic_ratio"
	"github.com/NookMux/NookMux/internal/httpapi/controller/group"
	"github.com/NookMux/NookMux/internal/httpapi/controller/log"
	"github.com/NookMux/NookMux/internal/httpapi/controller/misc"
	"github.com/NookMux/NookMux/internal/httpapi/controller/model"
	"github.com/NookMux/NookMux/internal/httpapi/controller/oauth"
	"github.com/NookMux/NookMux/internal/httpapi/controller/option"
	"github.com/NookMux/NookMux/internal/httpapi/controller/passkey"
	"github.com/NookMux/NookMux/internal/httpapi/controller/performance"
	"github.com/NookMux/NookMux/internal/httpapi/controller/prefill_group"
	"github.com/NookMux/NookMux/internal/httpapi/controller/rankings"
	"github.com/NookMux/NookMux/internal/httpapi/controller/redemption"
	"github.com/NookMux/NookMux/internal/httpapi/controller/secure_verification"
	"github.com/NookMux/NookMux/internal/httpapi/controller/setup"
	"github.com/NookMux/NookMux/internal/httpapi/controller/stored_media"
	"github.com/NookMux/NookMux/internal/httpapi/controller/ticket"
	"github.com/NookMux/NookMux/internal/httpapi/controller/token"
	"github.com/NookMux/NookMux/internal/httpapi/controller/topup"
	"github.com/NookMux/NookMux/internal/httpapi/controller/twofa"
	"github.com/NookMux/NookMux/internal/httpapi/controller/uptime_kuma"
	"github.com/NookMux/NookMux/internal/httpapi/controller/usedata"
	"github.com/NookMux/NookMux/internal/httpapi/controller/user"
	"github.com/NookMux/NookMux/internal/httpapi/controller/vendor_meta"
	"github.com/NookMux/NookMux/internal/httpapi/middleware"

	// Import oauth package to register providers via init()
	_ "github.com/NookMux/NookMux/internal/oauth"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.BodyStorageCleanup()) // 清理请求体存储
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	anonymousRequestBodyLimit := middleware.AnonymousRequestBodyLimit()
	{
		apiRouter.GET("/setup", setupcontroller.GetSetup)
		apiRouter.POST("/setup", anonymousRequestBodyLimit, setupcontroller.PostSetup)
		apiRouter.GET("/status", misccontroller.GetStatus)
		apiRouter.GET("/status/admin_modules", middleware.AdminAuth(), misccontroller.GetStatusAdminModules)
		// 构建版本指纹属于管理语义，仅 AdminAuth 可见，不进匿名 /api/status
		apiRouter.GET("/status/system_info", middleware.AdminAuth(), misccontroller.GetStatusSystemInfo)
		// 面向用户的侧栏模块开关（chat/console/personal/support），登录即可见；
		// 管理段由服务端剥离，仅 admin_modules（AdminAuth）下发。
		// 响应内容随调用者角色变化，必须 no-store 防止管理员响应被共享缓存重放
		apiRouter.GET("/status/user_modules", middleware.UserAuth(), middleware.DisableCache(), misccontroller.GetStatusUserModules)
		// uptime 状态会触发服务端向 Uptime Kuma 实例的内网请求，
		// 必须登录后才可触发，避免匿名用户借其探测内网
		apiRouter.GET("/uptime/status", middleware.UserAuth(), uptimekumacontroller.GetUptimeKumaStatus)
		apiRouter.GET("/models", middleware.UserAuth(), modelcontroller.DashboardListModels)
		apiRouter.GET("/status/test", middleware.AdminAuth(), misccontroller.TestStatus)
		apiRouter.GET("/notice", misccontroller.GetNotice)
		apiRouter.GET("/user-agreement", misccontroller.GetUserAgreement)
		apiRouter.GET("/privacy-policy", misccontroller.GetPrivacyPolicy)
		apiRouter.GET("/about", misccontroller.GetAbout)
		apiRouter.GET("/home_page_content", misccontroller.GetHomePageContent)
		apiRouter.GET("/pricing", middleware.HeaderNavModuleAuth("pricing"), billingcontroller.GetPricing)
		apiRouter.GET("/rankings", middleware.HeaderNavModuleAuth("rankings"), rankingscontroller.GetRankings)
		apiRouter.GET("/verification", middleware.EmailVerificationRateLimit(), middleware.TurnstileCheck(), misccontroller.SendEmailVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), misccontroller.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, misccontroller.ResetPassword)
		// OAuth routes - specific routes must come before :provider wildcard
		apiRouter.GET("/oauth/state", middleware.CriticalRateLimit(), oauthcontroller.GenerateOAuthCode)
		apiRouter.POST("/oauth/email/bind", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, usercontroller.EmailBind)
		// Standard OAuth providers (GitHub, LinuxDO) - unified route
		apiRouter.GET("/oauth/:provider", middleware.CriticalRateLimit(), oauthcontroller.HandleOAuth)

		apiRouter.POST("/stripe/webhook", anonymousRequestBodyLimit, topupcontroller.StripeWebhook)

		// Universal secure verification routes
		apiRouter.POST("/verify", middleware.UserAuth(), middleware.CriticalRateLimit(), secureverificationcontroller.UniversalVerify)

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/register", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), usercontroller.Register)
			userRoute.POST("/login", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), usercontroller.Login)
			userRoute.POST("/login/2fa", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, twofacontroller.Verify2FALogin)
			userRoute.POST("/passkey/login/begin", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, passkeycontroller.PasskeyLoginBegin)
			userRoute.POST("/passkey/login/finish", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, passkeycontroller.PasskeyLoginFinish)
			//userRoute.POST("/tokenlog", middleware.CriticalRateLimit(), logcontroller.TokenLog)
			userRoute.GET("/logout", usercontroller.Logout)
			userRoute.POST("/epay/notify", anonymousRequestBodyLimit, topupcontroller.EpayNotify)
			userRoute.GET("/epay/notify", topupcontroller.EpayNotify)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.UserAuth())
			{
				selfRoute.GET("/self/groups", groupcontroller.GetUserGroups)
				selfRoute.GET("/self", usercontroller.GetSelf)
				selfRoute.GET("/models", usercontroller.GetUserModels)
				selfRoute.GET("/self/usage_log_fields", misccontroller.GetUsageLogFieldsVisible)
				selfRoute.PUT("/self", usercontroller.UpdateSelf)
				selfRoute.DELETE("/self", usercontroller.DeleteSelf)
				selfRoute.GET("/token", usercontroller.GenerateAccessToken)
				selfRoute.GET("/passkey", passkeycontroller.PasskeyStatus)
				selfRoute.POST("/passkey/register/begin", passkeycontroller.PasskeyRegisterBegin)
				selfRoute.POST("/passkey/register/finish", passkeycontroller.PasskeyRegisterFinish)
				selfRoute.POST("/passkey/verify/begin", passkeycontroller.PasskeyVerifyBegin)
				selfRoute.POST("/passkey/verify/finish", passkeycontroller.PasskeyVerifyFinish)
				selfRoute.PUT("/passkey/:id", passkeycontroller.PasskeyUpdate)
				selfRoute.DELETE("/passkey/:id", passkeycontroller.PasskeyDelete)
				selfRoute.DELETE("/passkey", passkeycontroller.PasskeyDelete)
				selfRoute.GET("/aff", usercontroller.GetAffCode)
				selfRoute.GET("/topup/info", topupcontroller.GetTopUpInfo)
				selfRoute.GET("/topup/self", topupcontroller.GetUserTopUps)
				selfRoute.POST("/topup", middleware.CriticalRateLimit(), usercontroller.TopUp)
				selfRoute.POST("/pay", middleware.CriticalRateLimit(), topupcontroller.RequestEpay)
				selfRoute.POST("/amount", topupcontroller.RequestAmount)
				selfRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), topupcontroller.RequestStripePay)
				selfRoute.POST("/stripe/amount", topupcontroller.RequestStripeAmount)
				selfRoute.POST("/aff_transfer", usercontroller.TransferAffQuota)
				selfRoute.PUT("/setting", usercontroller.UpdateUserSetting)

				// 2FA routes
				selfRoute.GET("/2fa/status", twofacontroller.Get2FAStatus)
				selfRoute.POST("/2fa/setup", twofacontroller.Setup2FA)
				selfRoute.POST("/2fa/enable", twofacontroller.Enable2FA)
				selfRoute.POST("/2fa/disable", twofacontroller.Disable2FA)
				selfRoute.POST("/2fa/backup_codes", twofacontroller.RegenerateBackupCodes)

				// Check-in routes
				selfRoute.GET("/checkin", checkincontroller.GetCheckinStatus)
				selfRoute.POST("/checkin", middleware.TurnstileCheck(), checkincontroller.DoCheckin)

			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth())
			{
				adminRoute.GET("/", usercontroller.GetAllUsers)
				adminRoute.GET("/topup", topupcontroller.GetAllTopUps)
				adminRoute.POST("/topup/complete", topupcontroller.AdminCompleteTopUp)
				adminRoute.GET("/search", usercontroller.SearchUsers)
				adminRoute.GET("/:id", usercontroller.GetUser)
				adminRoute.POST("/", usercontroller.CreateUser)
				adminRoute.POST("/manage", usercontroller.ManageUser)
				adminRoute.PUT("/", usercontroller.UpdateUser)
				adminRoute.DELETE("/:id", usercontroller.DeleteUser)
				adminRoute.DELETE("/:id/reset_passkey", passkeycontroller.AdminResetPasskey)

				// Admin 2FA routes
				adminRoute.GET("/2fa/stats", twofacontroller.Admin2FAStats)
				adminRoute.DELETE("/:id/2fa", twofacontroller.AdminDisable2FA)
			}
		}

		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth())
		{
			optionRoute.GET("/", optioncontroller.GetOptions)
			optionRoute.GET("/value", optioncontroller.GetOptionValue)
			optionRoute.GET("/json_map", optioncontroller.GetOptionJsonMap)
			optionRoute.GET("/json_array", optioncontroller.GetOptionJsonArray)
			optionRoute.PUT("/", optioncontroller.UpdateOption)
			optionRoute.PUT("/json_map", optioncontroller.UpsertOptionJsonMapEntry)
			optionRoute.PUT("/json_array", optioncontroller.UpsertOptionJsonArrayEntry)
			optionRoute.DELETE("/json_map", optioncontroller.DeleteOptionJsonMapEntry)
			optionRoute.DELETE("/json_array", optioncontroller.DeleteOptionJsonArrayEntry)
			optionRoute.GET("/channel_affinity_cache", channelcontroller.GetChannelAffinityCacheStats)
			optionRoute.DELETE("/channel_affinity_cache", channelcontroller.ClearChannelAffinityCache)
			optionRoute.POST("/rest_model_ratio", billingcontroller.ResetModelRatio)
			optionRoute.POST("/reset_tool_billing_rules", billingcontroller.ResetToolBillingRules)
			optionRoute.POST("/migrate_console_setting", optioncontroller.MigrateConsoleSetting) // 用于迁移检测的旧键，下个版本会删除
		}

		dbRoute := apiRouter.Group("/db")
		dbRoute.Use(middleware.RootAuth())
		{
			dbRoute.GET("/pre_migrate/info", dbmigratecontroller.GetDBPreMigrateInfo)
			dbRoute.POST("/pre_migrate", middleware.CriticalRateLimit(), dbmigratecontroller.StartDBPreMigrate)
			dbRoute.GET("/pre_migrate/:id", dbmigratecontroller.GetDBPreMigrateJob)
			dbRoute.GET("/same_type_migrate/info", dbmigratecontroller.GetDBSameTypeMigrateInfo)
			dbRoute.POST("/same_type_migrate", middleware.CriticalRateLimit(), dbmigratecontroller.StartDBSameTypeMigrate)
			dbRoute.GET("/same_type_migrate/:id", dbmigratecontroller.GetDBSameTypeMigrateJob)
		}

		performanceRoute := apiRouter.Group("/performance")
		performanceRoute.Use(middleware.RootAuth())
		{
			performanceRoute.GET("/stats", performancecontroller.GetPerformanceStats)
			performanceRoute.DELETE("/disk_cache", performancecontroller.ClearDiskCache)
			performanceRoute.POST("/reset_stats", performancecontroller.ResetPerformanceStats)
			performanceRoute.POST("/gc", performancecontroller.ForceGC)
		}
		channelRoute := apiRouter.Group("/channel")
		channelRoute.Use(middleware.AdminAuth())
		{
			channelRoute.GET("/", channelcontroller.GetAllChannels)
			channelRoute.GET("/search", channelcontroller.SearchChannels)
			channelRoute.GET("/models", modelcontroller.ChannelListModels)
			channelRoute.GET("/models_enabled", modelcontroller.EnabledListModels)
			channelRoute.GET("/:id", channelcontroller.GetChannel)
			channelRoute.POST("/:id/key", middleware.RootAuth(), middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.SecureVerificationRequired(), channelcontroller.GetChannelKey)
			channelRoute.GET("/test", channelcontroller.TestAllChannels)
			channelRoute.GET("/test/:id", channelcontroller.TestChannel)
			channelRoute.GET("/update_balance", channelcontroller.UpdateAllChannelsBalance)
			channelRoute.GET("/update_balance/:id", channelcontroller.UpdateChannelBalance)
			channelRoute.POST("/", channelcontroller.AddChannel)
			channelRoute.PUT("/", channelcontroller.UpdateChannel)
			channelRoute.DELETE("/disabled", channelcontroller.DeleteDisabledChannel)
			channelRoute.POST("/tag/disabled", channelcontroller.DisableTagChannels)
			channelRoute.POST("/tag/enabled", channelcontroller.EnableTagChannels)
			channelRoute.PUT("/tag", channelcontroller.EditTagChannels)
			channelRoute.DELETE("/:id", channelcontroller.DeleteChannel)
			channelRoute.POST("/batch", channelcontroller.DeleteChannelBatch)
			channelRoute.POST("/fix", channelcontroller.FixChannelsAbilities)
			channelRoute.GET("/fetch_models/:id", middleware.RootAuth(), channelcontroller.FetchUpstreamModels)
			channelRoute.POST("/fetch_models", middleware.RootAuth(), channelcontroller.FetchModels)
			channelRoute.GET("/fetch_providers/:id", middleware.RootAuth(), channelcontroller.FetchUpstreamProviders)
			channelRoute.POST("/fetch_providers", middleware.RootAuth(), channelcontroller.FetchProviders)
			channelRoute.POST("/ollama/pull", channelcontroller.OllamaPullModel)
			channelRoute.POST("/ollama/pull/stream", channelcontroller.OllamaPullModelStream)
			channelRoute.DELETE("/ollama/delete", channelcontroller.OllamaDeleteModel)
			channelRoute.GET("/ollama/version/:id", channelcontroller.OllamaVersion)
			channelRoute.POST("/batch/tag", channelcontroller.BatchSetChannelTag)
			channelRoute.GET("/tag/models", channelcontroller.GetTagModels)
			channelRoute.GET("/builtin_urls", channelcontroller.GetBuiltinChannelURLs)
			channelRoute.POST("/copy/:id", channelcontroller.CopyChannel)
			channelRoute.POST("/multi_key/manage", channelcontroller.ManageMultiKeys)
			channelRoute.POST("/test_proxy", channelcontroller.TestProxy)
			channelRoute.GET("/plan/quota/:id", channelcontroller.QueryPlanQuota)
			channelRoute.GET("/plan/glm/usage/:id", channelcontroller.QueryGlmUsage)
			channelRoute.GET("/plan/glm/risk/:id", channelcontroller.QueryRiskStatus)
			channelRoute.GET("/glm/contact/:id", channelcontroller.QueryGlmContactInfo)
			channelRoute.GET("/glm/account_report/:id", channelcontroller.QueryGlmAccountReport)
			channelRoute.GET("/plan/glm/activity/:id", channelcontroller.QueryGlmPlanActivity)
			channelRoute.GET("/plan/glm/reset_cards/:id", channelcontroller.QueryGlmResetCards)
			channelRoute.POST("/plan/glm/reset_cards/:id/use", channelcontroller.UseGlmResetCard)
		}
		tokenRoute := apiRouter.Group("/token")
		tokenRoute.Use(middleware.UserAuth())
		{
			tokenRoute.GET("/", tokencontroller.GetAllTokens)
			tokenRoute.GET("/search", middleware.SearchRateLimit(), tokencontroller.SearchTokens)
			tokenRoute.GET("/:id", tokencontroller.GetToken)
			tokenRoute.POST("/", tokencontroller.AddToken)
			tokenRoute.PUT("/", tokencontroller.UpdateToken)
			tokenRoute.DELETE("/:id", tokencontroller.DeleteToken)
			tokenRoute.POST("/batch", tokencontroller.DeleteTokenBatch)
			tokenRoute.POST("/batch/keys", tokencontroller.GetTokenKeysBatch)
			tokenRoute.POST("/:id/key", middleware.CriticalRateLimit(), tokencontroller.GetTokenKey)
			tokenRoute.PUT("/:id/key", middleware.CriticalRateLimit(), tokencontroller.ResetTokenKey)
		}

		ticketRoute := apiRouter.Group("/ticket")
		ticketRoute.Use(middleware.UserAuth())
		{
			ticketRoute.GET("/", ticketcontroller.GetUserTickets)
			// 管理工单列表在路由层收紧到 AdminAuth；
			// service.ListAdminTickets 内部的角色检查保留作纵深防御
			ticketRoute.GET("/admin", middleware.AdminAuth(), ticketcontroller.GetAdminTickets)
			ticketRoute.POST("/", ticketcontroller.CreateTicket)
			ticketRoute.GET("/:id", ticketcontroller.GetTicketDetail)
			ticketRoute.POST("/:id/reply", ticketcontroller.ReplyTicket)
			ticketRoute.POST("/:id/close", ticketcontroller.CloseTicket)
			ticketRoute.POST("/:id/status", ticketcontroller.UpdateTicketStatus)
		}

		usageRoute := apiRouter.Group("/usage")
		usageRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			tokenUsageRoute := usageRoute.Group("/token")
			tokenUsageRoute.Use(middleware.TokenAuthReadOnly())
			{
				tokenUsageRoute.GET("/", tokencontroller.GetTokenUsage)
			}
		}

		redemptionRoute := apiRouter.Group("/redemption")
		redemptionRoute.Use(middleware.AdminAuth())
		{
			redemptionRoute.GET("/", redemptioncontroller.GetAllRedemptions)
			redemptionRoute.GET("/search", redemptioncontroller.SearchRedemptions)
			redemptionRoute.GET("/:id", redemptioncontroller.GetRedemption)
			// 完整兑换码 key 的按需查看响应必须 no-store，避免浏览器/中间代理缓存
			redemptionRoute.GET("/:id/key", middleware.DisableCache(), redemptioncontroller.GetRedemptionKey)
			redemptionRoute.POST("/", redemptioncontroller.AddRedemption)
			redemptionRoute.PUT("/", redemptioncontroller.UpdateRedemption)
			redemptionRoute.DELETE("/invalid", redemptioncontroller.DeleteInvalidRedemption)
			redemptionRoute.DELETE("/:id", redemptioncontroller.DeleteRedemption)
		}
		logRoute := apiRouter.Group("/log")
		logRoute.GET("/", middleware.AdminAuth(), logcontroller.GetAllLogs)
		logRoute.DELETE("/", middleware.AdminAuth(), logcontroller.DeleteHistoryLogs)
		logRoute.GET("/stat", middleware.AdminAuth(), logcontroller.GetLogsStat)
		logRoute.GET("/self/stat", middleware.UserAuth(), logcontroller.GetLogsSelfStat)
		logRoute.GET("/channel_affinity_usage_cache", middleware.AdminAuth(), channelcontroller.GetChannelAffinityUsageCacheStats)
		logRoute.GET("/self", middleware.UserAuth(), logcontroller.GetUserLogs)

		dataRoute := apiRouter.Group("/data")
		dataRoute.GET("/", middleware.AdminAuth(), usedatacontroller.GetAllQuotaDates)
		dataRoute.GET("/users", middleware.AdminAuth(), usedatacontroller.GetQuotaDataGroupByUser)
		dataRoute.GET("/self", middleware.UserAuth(), usedatacontroller.GetUserQuotaDates)
		dataRoute.GET("/media_convert_stats", middleware.AdminAuth(), usedatacontroller.GetAllMediaConvertStats)
		dataRoute.GET("/self/media_convert_stats", middleware.UserAuth(), usedatacontroller.GetUserMediaConvertStats)
		dataRoute.POST("/recalculate", middleware.AdminAuth(), usedatacontroller.RecalculateQuotaData)

		dashboardRoute := apiRouter.Group("/dashboard")
		dashboardRoute.Use(middleware.AdminAuth())
		{
			dashboardRoute.GET("/config", optioncontroller.GetDashboardConfig)
			dashboardRoute.PUT("/config", optioncontroller.UpdateDashboardConfig)
			dashboardRoute.POST("/config/reset", optioncontroller.ResetDashboardConfig)
		}

		logRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			logRoute.GET("/token", middleware.TokenAuthReadOnly(), logcontroller.GetLogByKey)
			logRoute.GET("/token/usage_log_fields", middleware.TokenAuthReadOnly(), misccontroller.GetUsageLogFieldsVisible)
		}

		auditRoute := apiRouter.Group("/audit")
		auditRoute.Use(middleware.AdminAuth())
		{
			auditRoute.GET("/", auditcontroller.GetAuditLogs)
			auditRoute.GET("/modules", auditcontroller.GetAuditModules)
		}

		storedMediaRoute := apiRouter.Group("/stored_media")
		{
			storedMediaRoute.GET("/", middleware.AdminAuth(), storedmediacontroller.GetAllStoredMedia)
			storedMediaRoute.GET("/self", middleware.UserAuth(), storedmediacontroller.GetSelfStoredMedia)
			storedMediaRoute.GET("/:media_type/:id", middleware.UserAuth(), storedmediacontroller.GetStoredMediaDetail)
			storedMediaRoute.DELETE("/:media_type/:id", middleware.UserAuth(), storedmediacontroller.DeleteStoredMedia)
			storedMediaRoute.POST("/batch", middleware.UserAuth(), storedmediacontroller.DeleteStoredMediaBatch)
		}
		groupRoute := apiRouter.Group("/group")
		groupRoute.Use(middleware.AdminAuth())
		{
			groupRoute.GET("/", groupcontroller.GetGroups)
		}

		prefillGroupRoute := apiRouter.Group("/prefill_group")
		prefillGroupRoute.Use(middleware.AdminAuth())
		{
			prefillGroupRoute.GET("/", prefillgroupcontroller.GetPrefillGroups)
			prefillGroupRoute.POST("/", prefillgroupcontroller.CreatePrefillGroup)
			prefillGroupRoute.PUT("/", prefillgroupcontroller.UpdatePrefillGroup)
			prefillGroupRoute.DELETE("/:id", prefillgroupcontroller.DeletePrefillGroup)
		}

		vendorRoute := apiRouter.Group("/vendors")
		vendorRoute.Use(middleware.AdminAuth())
		{
			vendorRoute.GET("/", vendormetacontroller.GetAllVendors)
			vendorRoute.GET("/search", vendormetacontroller.SearchVendors)
			vendorRoute.GET("/:id", vendormetacontroller.GetVendorMeta)
			vendorRoute.POST("/", vendormetacontroller.CreateVendorMeta)
			vendorRoute.PUT("/", vendormetacontroller.UpdateVendorMeta)
			vendorRoute.DELETE("/:id", vendormetacontroller.DeleteVendorMeta)
		}

		modelsRoute := apiRouter.Group("/models")
		modelsRoute.Use(middleware.AdminAuth())
		{
			modelsRoute.GET("/sync_upstream/preview", modelcontroller.SyncUpstreamPreview)
			modelsRoute.POST("/sync_upstream", modelcontroller.SyncUpstreamModels)
			modelsRoute.GET("/missing", channelcontroller.GetMissingModels)
			modelsRoute.GET("/", modelcontroller.GetAllModelsMeta)
			modelsRoute.GET("/search", modelcontroller.SearchModelsMeta)
			modelsRoute.GET("/:id", modelcontroller.GetModelMeta)
			modelsRoute.POST("/", modelcontroller.CreateModelMeta)
			modelsRoute.PUT("/", modelcontroller.UpdateModelMeta)
			modelsRoute.DELETE("/:id", modelcontroller.DeleteModelMeta)
		}

		dynamicRatioRoute := apiRouter.Group("/dynamic_ratio")
		{
			dynamicRatioRoute.GET("/status", middleware.UserAuth(), dynamicratiocontroller.GetDynamicRatioStatus)
			dynamicRatioRoute.GET("/rules", middleware.AdminAuth(), dynamicratiocontroller.GetDynamicRatioRules)
			dynamicRatioRoute.POST("/rules", middleware.RootAuth(), dynamicratiocontroller.CreateDynamicRatioRule)
			dynamicRatioRoute.PUT("/rules", middleware.RootAuth(), dynamicratiocontroller.UpdateDynamicRatioRule)
			dynamicRatioRoute.DELETE("/rules/:id", middleware.RootAuth(), dynamicratiocontroller.DeleteDynamicRatioRule)
			dynamicRatioRoute.PUT("/rules/reorder", middleware.RootAuth(), dynamicratiocontroller.ReorderDynamicRatioRules)
			dynamicRatioRoute.PUT("/enabled", middleware.RootAuth(), dynamicratiocontroller.SetDynamicRatioEnabled)
		}

		// 定制音色（用户侧）：上传试听 + 确认定制
		customVoiceRoute := apiRouter.Group("/custom_voice")
		customVoiceRoute.Use(middleware.UserAuth())
		{
			customVoiceRoute.GET("/tags", customvoicecontroller.CustomVoiceTagsHandler)
			customVoiceRoute.POST("/preview", customvoicecontroller.CustomVoicePreviewHandler)
			customVoiceRoute.POST("/confirm_quote", customvoicecontroller.CustomVoiceConfirmQuoteHandler)
			customVoiceRoute.POST("/confirm", customvoicecontroller.CustomVoiceConfirmHandler)
		}

		// 音色管理（管理员）：列表/新增（Admin），修改/删除（Root）
		minimaxVoiceRoute := apiRouter.Group("/minimax/voices")
		{
			minimaxVoiceRoute.GET("/", middleware.AdminAuth(), customvoicecontroller.GetMiniMaxVoices)
			minimaxVoiceRoute.POST("/", middleware.AdminAuth(), customvoicecontroller.CreateMiniMaxVoice)
			minimaxVoiceRoute.PUT("/:id", middleware.RootAuth(), customvoicecontroller.UpdateMiniMaxVoice)
			minimaxVoiceRoute.DELETE("/:id", middleware.RootAuth(), customvoicecontroller.DeleteMiniMaxVoice)
		}
	}
}
