package dbmigrate

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"time"
)

const (
	dbPreMigrateBatchDefault = 500
	dbPreMigrateBatchBlob    = 100
	dbPreMigrateBatchLog     = 2000
)

type dbPreMigrateStep interface {
	Name() string
	Run(ctx context.Context, job *DBPreMigrateJob, src *gorm.DB, dst *gorm.DB, params DBPreMigrateStartParams) error
}

type gormTableCopyStep[T any] struct {
	name      string
	batchSize int
}

func (s gormTableCopyStep[T]) Name() string { return s.name }

func (s gormTableCopyStep[T]) Run(ctx context.Context, job *DBPreMigrateJob, src *gorm.DB, dst *gorm.DB, params DBPreMigrateStartParams) error {
	job.setStep("迁移表：" + s.name)
	job.appendLog(fmt.Sprintf("[%s] 开始迁移表 %s", time.Now().Format(time.RFC3339), s.name))

	copied, total, err := copyTable[T](ctx, src, dst, copyTableOptions{
		ClearDst:  params.Force,
		BatchSize: s.batchSize,
	}, func(done int64, all int64) {
		job.setTableProgress(s.name, done, all)
	})
	job.setTableProgress(s.name, copied, total)
	if err != nil {
		job.appendLog(fmt.Sprintf("[%s] 迁移表 %s 失败：%v", time.Now().Format(time.RFC3339), s.name, err))
		return err
	}
	job.appendLog(fmt.Sprintf("[%s] 迁移表 %s 完成：%d/%d", time.Now().Format(time.RFC3339), s.name, copied, total))
	return nil
}

type copyTableOptions struct {
	ClearDst  bool
	BatchSize int
}

func copyTable[T any](ctx context.Context, src *gorm.DB, dst *gorm.DB, opts copyTableOptions, onProgress func(done int64, total int64)) (int64, int64, error) {
	if src == nil || dst == nil {
		return 0, 0, fmt.Errorf("src/dst 数据库连接不能为空")
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = dbPreMigrateBatchDefault
	}

	modelPtr := new(T)
	var total int64
	if err := src.WithContext(ctx).Unscoped().Model(modelPtr).Count(&total).Error; err != nil {
		return 0, 0, err
	}

	if opts.ClearDst {
		if err := clearTable(dst.WithContext(ctx), modelPtr); err != nil {
			return 0, total, err
		}
	}

	var copied int64
	var batch []T
	err := src.WithContext(ctx).Unscoped().Model(modelPtr).FindInBatches(&batch, opts.BatchSize, func(tx *gorm.DB, _ int) error {
		if ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if len(batch) == 0 {
			return nil
		}
		// 每批写入包在独立事务中：将整批语句钉定在同一连接上执行（达梦 IDENTITY_INSERT
		// 的 SET/INSERT/SET OFF 依赖会话粘性），并保证批级原子性、减少 commit 次数。
		if err := dst.WithContext(ctx).Transaction(func(dtx *gorm.DB) error {
			return dtx.Session(&gorm.Session{CreateBatchSize: opts.BatchSize}).Create(&batch).Error
		}); err != nil {
			return err
		}
		copied += int64(len(batch))
		if onProgress != nil {
			onProgress(copied, total)
		}
		return nil
	}).Error
	if err != nil {
		return copied, total, err
	}
	if onProgress != nil {
		onProgress(copied, total)
	}
	return copied, total, nil
}

func clearTable(db *gorm.DB, model any) error {
	if db == nil {
		return fmt.Errorf("db 不能为空")
	}
	if model == nil {
		return fmt.Errorf("model 不能为空")
	}
	return db.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(model).Error
}
