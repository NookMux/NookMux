package dbmigrate

import (
	"database/sql"
	"fmt"
	"gorm.io/gorm"
	"strings"
)

type pgOwnedSequence struct {
	SchemaName   string `gorm:"column:schema_name"`
	SequenceName string `gorm:"column:sequence_name"`
	TableName    string `gorm:"column:table_name"`
	ColumnName   string `gorm:"column:column_name"`
}

func resetPostgresOwnedSequences(db *gorm.DB) (int, error) {
	seqs, err := listPostgresOwnedSequences(db)
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, seq := range seqs {
		if err := resetOnePostgresOwnedSequence(db, seq); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}

func listPostgresOwnedSequences(db *gorm.DB) ([]pgOwnedSequence, error) {
	// relkind：'S' 为 PostgreSQL 序列，'z' 为 openGauss 序列（PG 兼容内核的私有取值）。
	const q = `
SELECT
  ns.nspname AS schema_name,
  s.relname AS sequence_name,
  t.relname AS table_name,
  a.attname AS column_name
FROM pg_class s
JOIN pg_namespace ns ON ns.oid = s.relnamespace
JOIN pg_depend d ON d.objid = s.oid
JOIN pg_class t ON t.oid = d.refobjid
JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = d.refobjsubid
WHERE s.relkind IN ('S', 'z') AND d.deptype = 'a'
ORDER BY ns.nspname, t.relname, a.attname`

	var seqs []pgOwnedSequence
	if err := db.Raw(q).Scan(&seqs).Error; err != nil {
		return nil, err
	}
	return seqs, nil
}

func resetOnePostgresOwnedSequence(db *gorm.DB, seq pgOwnedSequence) error {
	maxVal, hasValue, err := maxPostgresIntColumn(db, seq.SchemaName, seq.TableName, seq.ColumnName)
	if err != nil {
		return fmt.Errorf("获取最大值失败：%s.%s.%s: %w", seq.SchemaName, seq.TableName, seq.ColumnName, err)
	}
	setTo := int64(1)
	isCalled := false
	if hasValue {
		setTo = maxVal
		isCalled = true
	}
	fullSeq := fmt.Sprintf("%s.%s", seq.SchemaName, seq.SequenceName)
	return db.Exec("SELECT setval(?::regclass, ?, ?)", fullSeq, setTo, isCalled).Error
}

func maxPostgresIntColumn(db *gorm.DB, schema string, table string, column string) (int64, bool, error) {
	sqlText := fmt.Sprintf(
		"SELECT MAX(%s) AS max_val FROM %s.%s",
		quotePostgresIdentifier(column),
		quotePostgresIdentifier(schema),
		quotePostgresIdentifier(table),
	)
	var v sql.NullInt64
	if err := db.Raw(sqlText).Scan(&v).Error; err != nil {
		return 0, false, err
	}
	if !v.Valid {
		return 0, false, nil
	}
	return v.Int64, true, nil
}

func quotePostgresIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
