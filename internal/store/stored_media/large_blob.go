package storedmediastore

import (
	"database/sql/driver"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// LargeBlob is a cross-database "large binary" type.
//
// - MySQL:     LONGBLOB (up to 4GB)
// - PostgreSQL: BYTEA
// - SQLite:    BLOB
//
// This avoids MySQL's default BLOB (64KB) limitation for media storage while
// keeping the schema compatible across SQLite/MySQL/PostgreSQL.
type LargeBlob []byte

func (LargeBlob) GormDataType() string {
	return "large_blob"
}

func (LargeBlob) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "mysql":
		return "LONGBLOB"
	case "postgres":
		return "BYTEA"
	case "sqlite":
		return "BLOB"
	default:
		return "BLOB"
	}
}

func (b LargeBlob) Value() (driver.Value, error) {
	if b == nil {
		return nil, nil
	}
	return []byte(b), nil
}

func (b *LargeBlob) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*b = nil
		return nil
	case []byte:
		buf := make([]byte, len(v))
		copy(buf, v)
		*b = LargeBlob(buf)
		return nil
	case string:
		*b = LargeBlob([]byte(v))
		return nil
	default:
		return fmt.Errorf("unsupported scan type for LargeBlob: %T", value)
	}
}
