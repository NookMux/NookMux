package dbmigrate

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/stored_media"
)

// 旧版"图片/视频转 URL"分表存储：stored_images 与 stored_videos，已合并为
// stored_media 单表（media_type 列区分图片/视频）。
const (
	legacyStoredImagesTable = "stored_images"
	legacyStoredVideosTable = "stored_videos"
)

// 单批合并行数与 premigrate 框架的 blob 表批次一致，避免单次事务过大。
const mergeStoredMediaBatchSize = 100

// mergeLegacyStoredMediaTables 把旧版 stored_images / stored_videos 两张同构表
// 的数据并入 stored_media，随后删除旧表。必须在
// AutoMigrate(&storedmediastore.StoredMedia{}) 之后执行，确保目标表与
// media_type 列已就绪。幂等可重复运行：旧表不存在时直接跳过；拷贝按 id 分批
// 且跳过已存在的 id，进程中断后重启可安全续跑。
func mergeLegacyStoredMediaTables() error {
	if err := mergeLegacyStoredMediaTable(legacyStoredImagesTable, storedmediastore.MediaTypeImage); err != nil {
		return err
	}
	return mergeLegacyStoredMediaTable(legacyStoredVideosTable, storedmediastore.MediaTypeVideo)
}

func mergeLegacyStoredMediaTable(legacyTable string, mediaType string) error {
	m := dbstore.DB.Migrator()
	if !m.HasTable(legacyTable) {
		return nil
	}
	if !m.HasTable(&storedmediastore.StoredMedia{}) {
		return fmt.Errorf("表 %s 存在但 stored_media 不存在，请先完成 AutoMigrate 再执行合并", legacyTable)
	}

	var copied int64
	var cursor string
	for {
		var rows []storedmediastore.StoredMedia
		if err := dbstore.DB.Table(legacyTable).
			Select("id", "user_id", "channel_id", "created_at", "mime_type", "size_bytes", "sha256", "data").
			Where("id > ?", cursor).
			Order("id asc").
			Limit(mergeStoredMediaBatchSize).
			Find(&rows).Error; err != nil {
			return fmt.Errorf("读取旧表 %s 失败: %w", legacyTable, err)
		}
		if len(rows) == 0 {
			break
		}

		ids := make([]string, 0, len(rows))
		for i := range rows {
			rows[i].MediaType = mediaType
			ids = append(ids, rows[i].Id)
		}
		existing, err := findExistingStoredMediaIds(ids)
		if err != nil {
			return fmt.Errorf("查询 stored_media 已有记录失败（来自 %s）: %w", legacyTable, err)
		}

		pending := make([]storedmediastore.StoredMedia, 0, len(rows))
		for i := range rows {
			if _, ok := existing[rows[i].Id]; !ok {
				pending = append(pending, rows[i])
			}
		}
		if len(pending) > 0 {
			if err := dbstore.DB.CreateInBatches(&pending, len(pending)).Error; err != nil {
				return fmt.Errorf("写入 stored_media 失败（来自 %s）: %w", legacyTable, err)
			}
			copied += int64(len(pending))
		}

		cursor = rows[len(rows)-1].Id
	}

	// 终态校验：旧表所有行都必须已存在于 stored_media，再删除旧表。
	// 表名来自包内常量，不拼接外部输入。
	var missing int64
	if err := dbstore.DB.Raw(fmt.Sprintf(
		"SELECT COUNT(1) FROM %s l WHERE NOT EXISTS (SELECT 1 FROM stored_media t WHERE t.id = l.id)",
		legacyTable,
	)).Scan(&missing).Error; err != nil {
		return fmt.Errorf("校验旧表 %s 合并结果失败: %w", legacyTable, err)
	}
	if missing > 0 {
		return fmt.Errorf("表 %s 仍有 %d 行未合并到 stored_media，已中止删除旧表", legacyTable, missing)
	}

	if err := m.DropTable(legacyTable); err != nil {
		return fmt.Errorf("删除旧表 %s 失败: %w", legacyTable, err)
	}
	common.SysLog(fmt.Sprintf("merged legacy table %s into stored_media (%d rows migrated)", legacyTable, copied))
	return nil
}

func findExistingStoredMediaIds(ids []string) (map[string]struct{}, error) {
	var existing []string
	if err := dbstore.DB.Model(&storedmediastore.StoredMedia{}).
		Where("id IN ?", ids).
		Pluck("id", &existing).Error; err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(existing))
	for _, id := range existing {
		set[id] = struct{}{}
	}
	return set, nil
}
