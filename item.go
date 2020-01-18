package main

import (
	"github.com/jinzhu/gorm"
	"time"
)

type Item struct {
	Name string `gorm:"not null"`
	Price int
	Url string `gorm:"unique_index"`
}

type LatestItem struct {
	Item
	CreatedAt time.Time
}

type ItemMaster struct {
	gorm.Model
	Item
}

func (ItemMaster) TableName() string {
	return "item_master"
}
