package main

import (
	"github.com/jinzhu/gorm"
	"time"
)

type Item struct {
	Name string `gorm:"not null"`
	Price int
	Url string `gorm:"unique"`
}

type LatestItem struct {
	Item
	CreatedAt time.Time
}

func (LatestItem) TableName() string {
	return "latest_item"
}

type ItemMaster struct {
	gorm.Model
	Item
}

func (ItemMaster) TableName() string {
	return "item_master"
}

func (orgItem *ItemMaster) equals(targetItem ItemMaster) bool {
	return orgItem.Name == targetItem.Name && orgItem.Price == targetItem.Price && orgItem.Url == targetItem.Url
}
