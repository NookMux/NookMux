package dbmigratecontroller

import (
	"strings"
	"testing"

	"github.com/NookMux/NookMux/pkg/jsonx"
)

// TestPremigrateAuditAfterMasksDSNPassword 验证预迁移审计落库前，
// 请求副本中的目标库 DSN 口令已被遮蔽且保留非敏感骨架，
// 原始请求保持不变，迁移任务仍使用真实 DSN。
func TestPremigrateAuditAfterMasksDSNPassword(t *testing.T) {
	req := dbPreMigrateStartRequest{
		TargetDSN:    "postgres://root:s3cret@10.0.0.9:5432/nookmux",
		TargetLogDSN: "loguser:logpass@tcp(127.0.0.1:3306)/logs",
		IncludeLogs:  true,
	}

	auditRecord := premigrateAuditAfter(req)

	payload, err := jsonx.Marshal(auditRecord)
	if err != nil {
		t.Fatalf("marshal audit record: %v", err)
	}
	auditPayload := string(payload)
	for _, secret := range []string{"s3cret", "logpass"} {
		if strings.Contains(auditPayload, secret) {
			t.Fatalf("审计记录不应包含口令 %q: %s", secret, auditPayload)
		}
	}
	for _, skeleton := range []string{
		`"target_dsn":"postgres://root:***@10.0.0.9:5432/nookmux"`,
		`"target_log_dsn":"loguser:***@tcp(127.0.0.1:3306)/logs"`,
		`"include_logs":true`,
	} {
		if !strings.Contains(auditPayload, skeleton) {
			t.Fatalf("审计记录应保留非敏感骨架 %s: %s", skeleton, auditPayload)
		}
	}
	if !strings.Contains(req.TargetDSN, "s3cret") || !strings.Contains(req.TargetLogDSN, "logpass") {
		t.Fatalf("原始请求不应被修改: %+v", req)
	}
}

// TestSameTypeMigrateAuditAfterMasksDSNPassword 验证同类型迁移审计落库前，
// 请求副本中的目标库 DSN 口令已被遮蔽且保留非敏感骨架，
// 原始请求保持不变，迁移任务仍使用真实 DSN。
func TestSameTypeMigrateAuditAfterMasksDSNPassword(t *testing.T) {
	req := dbSameTypeMigrateStartRequest{
		TargetDSN:    "mysql-db:mainpass@tcp(192.168.1.10:3306)/nookmux?charset=utf8mb4",
		TargetLogDSN: "postgres://logs:logsecret@10.0.0.9:5432/logs",
		Force:        true,
	}

	auditRecord := sameTypeMigrateAuditAfter(req)

	payload, err := jsonx.Marshal(auditRecord)
	if err != nil {
		t.Fatalf("marshal audit record: %v", err)
	}
	auditPayload := string(payload)
	for _, secret := range []string{"mainpass", "logsecret"} {
		if strings.Contains(auditPayload, secret) {
			t.Fatalf("审计记录不应包含口令 %q: %s", secret, auditPayload)
		}
	}
	for _, skeleton := range []string{
		`"target_dsn":"mysql-db:***@tcp(192.168.1.10:3306)/nookmux?charset=utf8mb4"`,
		`"target_log_dsn":"postgres://logs:***@10.0.0.9:5432/logs"`,
		`"force":true`,
	} {
		if !strings.Contains(auditPayload, skeleton) {
			t.Fatalf("审计记录应保留非敏感骨架 %s: %s", skeleton, auditPayload)
		}
	}
	if !strings.Contains(req.TargetDSN, "mainpass") || !strings.Contains(req.TargetLogDSN, "logsecret") {
		t.Fatalf("原始请求不应被修改: %+v", req)
	}
}
