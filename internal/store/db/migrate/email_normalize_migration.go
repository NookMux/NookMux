package dbmigrate

import (
	"fmt"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/option"
)

// emailNormalizeBackfillMarker 一次性数据迁移标记（主库 options 表）。
const emailNormalizeBackfillMarker = "data_migration.email_normalized.v1.done"

// backfillEmailNormalization 把存量 users.email 统一规范化为小写去空白形
// 态，使各邮箱入口（绑定/找回密码/占用检查）的规范化比较对历史数据同样
// 成立。幂等：LOWER(TRIM(email)) 二次执行无变化；经 options marker 只完
// 整执行一次，失败不写 marker、下次启动重试。
//
// 若历史上存在仅大小写不同的多个账户，回填后它们成为等值重复行：密码
// 重置与绑定对这类行按歧义/占用拒绝（GetUniqueUserByEmail / IsEmail-
// AlreadyTaken），由管理员人工去重后恢复，不做静默合并。
func backfillEmailNormalization() {
	if optionstore.IsDataMigrationDone(emailNormalizeBackfillMarker) {
		return
	}
	result := dbstore.DB.Exec("UPDATE users SET email = LOWER(TRIM(email)) WHERE email <> LOWER(TRIM(email))")
	if result.Error != nil {
		common.SysError("email normalization backfill failed: " + result.Error.Error())
		return
	}
	if result.RowsAffected > 0 {
		common.SysLog(fmt.Sprintf("email normalization backfill updated %d rows", result.RowsAffected))
	}
	optionstore.MarkDataMigrationDone(emailNormalizeBackfillMarker)
}
