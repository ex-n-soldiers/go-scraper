package main

import (
	"github.com/jinzhu/gorm"
	"path/filepath"
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
	Description string
	LastCheckedAt time.Time
	ImageUrl string
	ImageLastModifiedAt time.Time
	ImageDownloadPath string
	PDFUrl string
	PDFLastModifiedAt time.Time
	PDFDownloadPath string
}

func (ItemMaster) TableName() string {
	return "item_master"
}

func (i ItemMaster) ImageFileName() string {
	return filepath.Base(i.ImageUrl)
}

func (i ItemMaster) PDFFileName() string {
	return filepath.Base(i.PDFUrl)
}
