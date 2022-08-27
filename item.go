package main

import (
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

type Item struct {
	Name  string `gorm:"not null"`
	Price int
	Url   string `gorm:"unique_index"`
}

type LatestItem struct {
	Item
	CreatedAt time.Time
}

type ItemMaster struct {
	gorm.Model
	Item
	Description         string
	ImageUrl            string
	ImageLastModifiedAt time.Time
	ImageDownloadPath   string
	PdfUrl              string
	PdfLastModifiedAt   time.Time
	PdfDownloadPath     string
}

type  HistoricalItem struct {
	Item
	CreatedAt time.Time
}

func (ItemMaster) TableName() string {
	return "item_master"
}

func (i ItemMaster) ImageFileName() string {
	return filepath.Base(i.ImageUrl)
}

func (i ItemMaster) PdfFileName() string {
	return filepath.Base(i.PdfUrl)
}

func (i ItemMaster) equals(target ItemMaster) bool {
	return i.Description == target.Description &&
		i.ImageUrl == target.ImageUrl &&
		i.ImageLastModifiedAt == target.ImageLastModifiedAt &&
		i.ImageDownloadPath == target.ImageDownloadPath &&
		i.PdfUrl == target.PdfUrl &&
		i.PdfLastModifiedAt == target.PdfLastModifiedAt &&
		i.PdfDownloadPath == target.PdfDownloadPath
}
