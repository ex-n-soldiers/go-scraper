package main

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func gormConnect(conf *Config) (*gorm.DB, error) {
	var dbHost = conf.Db.Host
	var dbPort = conf.Db.Port
	var dbName = conf.Db.DbName
	var dbUser = conf.Db.User
	var dbPassword = conf.Db.Password

	// URLとデータベースの環境変数を分離するため、fmt.Sprintfを使用
	dsn := fmt.Sprintf("%s:%s@(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", dbUser, dbPassword, dbHost, dbPort, dbName)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("db connection error: %w", err)
	}

	return db, nil
}

func dbMigration(db *gorm.DB) error {
	// item_masterテーブルとlatest_itemテーブルが作成される
	if err := db.AutoMigrate(&ItemMaster{}, &LatestItem{}); err != nil {
		return fmt.Errorf("db migration error: %w", err)
	}
	return nil
}
