package dbstore

import (
	"os"
	"testing"

	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestGetDBIdentityUnsupportedType 验证未适配的数据库类型在取得连接标识前即报错。
func TestGetDBIdentityUnsupportedType(t *testing.T) {
	_, err := GetDBIdentity(nil, "sqlite")
	require.Error(t, err)
}

// TestDBIdentityString 验证身份的日志表示：带监听地址的身份（MySQL）、
// 指纹+监听端口的身份（openGauss）与仅含实例指纹的身份（PostgreSQL 系）
// 各自保持可读。
func TestDBIdentityString(t *testing.T) {
	withAddr := dbIdentity{Host: "db1", Port: "5432", Database: "nookmux", ServerUUID: "728479703360"}
	assert.Equal(t, "db1:5432/nookmux (uuid=728479703360)", withAddr.String())

	openGaussForm := dbIdentity{Port: "5433", Database: "nookmux_pg", ServerUUID: "1792836225.7738"}
	assert.Equal(t, "nookmux_pg:5433 (instance=1792836225.7738)", openGaussForm.String())

	fingerprintOnly := dbIdentity{Database: "nookmux", ServerUUID: "7700326314263102029"}
	assert.Equal(t, "nookmux (instance=7700326314263102029)", fingerprintOnly.String())
}

// TestPostgresIdentityConnectionPathIndependent 验证 PostgreSQL/金仓身份仅由
// 实例指纹 + 库名构成（getPostgresIdentity 不采集 inet_server_addr()/
// inet_server_port() 连接路径字段）：同一实例经 IPv4 与 IPv6 连接产出完全
// 相等的 identity，不同实例或不同库产出不等。checkNotSameDB 以 == 比较
// 身份，据此判定同库/不同库。
func TestPostgresIdentityConnectionPathIndependent(t *testing.T) {
	viaIPv4 := dbIdentity{Database: "nookmux", ServerUUID: "7700326314263102029"}
	viaIPv6 := dbIdentity{Database: "nookmux", ServerUUID: "7700326314263102029"}
	assert.Equal(t, viaIPv4, viaIPv6, "同一实例跨地址族连接必须判为同一个库")

	otherInstance := dbIdentity{Database: "nookmux", ServerUUID: "7700326314263200000"}
	assert.NotEqual(t, viaIPv4, otherInstance, "不同实例必须判为不同库")

	otherDatabase := dbIdentity{Database: "nookmux_log", ServerUUID: "7700326314263102029"}
	assert.NotEqual(t, viaIPv4, otherDatabase, "同实例的不同库必须判为不同库")
}

// TestOpenGaussIdentityConnectionPathIndependent 验证 openGauss 身份由
// 实例指纹 + 监听端口 + 库名构成：同一实例经 IPv4 与 IPv6 连接
// （inet_server_addr() 分别取 127.0.0.1 与 ::1，inet_server_port() 恒同）
// 产出完全相等的 identity；启动时刻相同的两个实例由监听端口区分；不同库
// 产出不等。checkNotSameDB 以 == 比较身份，据此判定同库/不同库。
func TestOpenGaussIdentityConnectionPathIndependent(t *testing.T) {
	viaIPv4 := dbIdentity{Port: "5433", Database: "nookmux_pg", ServerUUID: "1792836225.7738"}
	viaIPv6 := dbIdentity{Port: "5433", Database: "nookmux_pg", ServerUUID: "1792836225.7738"}
	assert.Equal(t, viaIPv4, viaIPv6, "同一实例跨地址族连接必须判为同一个库")

	otherInstance := dbIdentity{Database: "nookmux_pg", ServerUUID: "1792836000.0000"}
	assert.NotEqual(t, viaIPv4, otherInstance, "不同实例必须判为不同库")

	sameStartInstantOtherPort := dbIdentity{Port: "6433", Database: "nookmux_pg", ServerUUID: "1792836225.7738"}
	assert.NotEqual(t, viaIPv4, sameStartInstantOtherPort, "同秒启动的不同实例由监听端口区分")

	otherDatabase := dbIdentity{Port: "5433", Database: "nookmux_log", ServerUUID: "1792836225.7738"}
	assert.NotEqual(t, viaIPv4, otherDatabase, "同实例的不同库必须判为不同库")
}

// TestGetOpenGaussIdentityAcrossAddressFamilies 在真实 openGauss 实例上验证
// 同一实例跨地址族连接取回的 identity 完全相等。设置
// OG_IDENTITY_TEST_DSN_V4 / OG_IDENTITY_TEST_DSN_V6（指向同一实例同一库、
// 仅地址族不同的 opengauss:// DSN）后生效，未设置时跳过。
func TestGetOpenGaussIdentityAcrossAddressFamilies(t *testing.T) {
	dsnV4 := os.Getenv("OG_IDENTITY_TEST_DSN_V4")
	dsnV6 := os.Getenv("OG_IDENTITY_TEST_DSN_V6")
	if dsnV4 == "" || dsnV6 == "" {
		t.Skip("未设置 OG_IDENTITY_TEST_DSN_V4/V6，跳过 openGauss 跨地址族身份验证")
	}
	idV4 := openGaussIdentityFromDSN(t, dsnV4)
	idV6 := openGaussIdentityFromDSN(t, dsnV6)
	assert.Equal(t, idV4, idV6, "同一实例经 IPv4/IPv6 连接必须判为同一个库")
	assert.NotEmpty(t, idV4.ServerUUID)
	assert.Equal(t, "nookmux_pg", idV4.Database)
}

func openGaussIdentityFromDSN(t *testing.T, dsn string) dbIdentity {
	t.Helper()
	g, err := OpenOpenGaussGorm(dsn, &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := g.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	id, err := GetDBIdentity(g, infradb.DatabaseTypeOpenGauss)
	require.NoError(t, err)
	return id
}
