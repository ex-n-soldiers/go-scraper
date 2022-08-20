package main

import (
	"fmt"

	"gorm.io/gorm"
)

func registerCurrentData(items []Item, db *gorm.DB) error {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(&LatestItem{}); err != nil {
		return fmt.Errorf("get latest_items table name error: %w", err)
	}
	if err := db.Exec("TRUNCATE " + stmt.Schema.Table).Error; err != nil {
		return fmt.Errorf("truncate latest_items error: %w", err)
	}

	var insertRecords []LatestItem
	for _, item := range items {
		insertRecords = append(insertRecords, LatestItem{Item: item})
	}
	if err := db.CreateInBatches(insertRecords, 100).Error; err != nil {
		return fmt.Errorf("bulk insert to latest_items error: %w", err)
	}

	return nil
}

func updateItemMaster(db *gorm.DB) error {
	// 整合性を保つために、データの追加、更新、削除はトランザクション内で実行
	return db.Transaction(func(tx *gorm.DB) error {
		// Insert
		var newItems []LatestItem
		err := tx.Unscoped().Joins("left join item_master on latest_items.url = item_master.url").Where("item_master.name is null").Find(&newItems).Error
		if err != nil {
			return fmt.Errorf("extract for bulk insert to item_master error: %w", err)
		}

		var insertRecords []ItemMaster
		for _, newItem := range newItems {
			insertRecords = append(insertRecords, ItemMaster{Item: newItem.Item})
			fmt.Printf("Index item is created: %s\n", newItem.Url)
		}
		if err := tx.CreateInBatches(insertRecords, 100).Error; err != nil {
			return fmt.Errorf("bulk insert to item_master error: %w", err)
		}

		// Update
		var updatedItems []LatestItem
		err = tx.Unscoped().Joins("inner join item_master on latest_items.url = item_master.url").Where("latest_items.name <> item_master.name or latest_items.price <> item_master.price or item_master.deleted_at is not null").Find(&updatedItems).Error
		if err != nil {
			return fmt.Errorf("update error: %w", err)
		}
		for _, updatedItem := range updatedItems {
			err := tx.Unscoped().Model(ItemMaster{}).Where("url = ?", updatedItem.Url).Updates(map[string]interface{}{"name": updatedItem.Name, "price": updatedItem.Price, "deleted_at": nil}).Error
			if err != nil {
				return fmt.Errorf("update error: %w", err)
			}
			fmt.Printf("Index item is updated: %s\n", updatedItem.Url)
		}

		// Delete
		var deletedItems []ItemMaster
		if err := tx.Where("not exists(select 1 from latest_items li where li.url = item_master.url)").Find(&deletedItems).Error; err != nil {
			return fmt.Errorf("delete error: %w", err)
		}
		var ids []uint
		for _, deletedItem := range deletedItems {
			ids = append(ids, deletedItem.ID)
			// 動作確認のために、ログを出力 本来はこのforループはなくしてもよい
			fmt.Printf("Index item is deleted: %s\n", deletedItem.Url)
		}
		if len(ids) > 0 {
			if err := tx.Delete(&deletedItems, ids).Error; err != nil {
				return fmt.Errorf("delete error: %w", err)
			}
		}

		return nil
	})
}

func getItemMasters(db *gorm.DB) ([]ItemMaster, error) {
	var items []ItemMaster
	if err := db.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("select error: %w", err)
	}

	return items, nil
}

func registerDetails(db *gorm.DB, items []ItemMaster) error {
	for _, item := range items {
		if err := db.Model(&item).Updates(ItemMaster{
			Description:         item.Description,
			ImageUrl:            item.ImageUrl,
			ImageLastModifiedAt: item.ImageLastModifiedAt,
			ImageDownloadPath:   item.ImageDownloadPath,
			PdfUrl:              item.PdfUrl,
			PdfLastModifiedAt:   item.PdfLastModifiedAt,
			PdfDownloadPath:     item.PdfDownloadPath}).Error; err != nil {
			return fmt.Errorf("update item detail info error: %w", err)
		}
		fmt.Printf("Detail page is updated: %s\n", item.Url)
	}
	return nil
}
