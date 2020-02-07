package main

import (
	"github.com/jinzhu/gorm"
	"path/filepath"
	"time"
)

type Item struct {
	Name string `gorm:"not null"`
	Price int
	URL string `gorm:"unique_index"`
}

type LatestItem struct {
	Item
	CreatedAt time.Time
}

type ItemMaster struct {
	gorm.Model
	Item
	Description string
	ImageURL string
	ImageLastModifiedAt time.Time
	ImageDownloadPath string
	PdfURL string
	PdfLastModifiedAt time.Time
	PdfDownloadPath string
}

func (ItemMaster) TableName() string {
	return "item_master"
}

func (i ItemMaster) ImageFileName() string {
	return filepath.Base(i.ImageURL)
}

func (i ItemMaster) PdfFileName() string {
	return filepath.Base(i.PdfURL)
}

func (i ItemMaster) equals(target ItemMaster) bool {
	return i.Description == target.Description &&
		i.ImageURL == target.ImageURL &&
		i.ImageLastModifiedAt == target.ImageLastModifiedAt &&
		i.ImageDownloadPath == target.ImageDownloadPath &&
		i.PdfURL == target.PdfURL &&
		i.PdfLastModifiedAt == target.PdfLastModifiedAt &&
		i.PdfDownloadPath == target.PdfDownloadPath
}

type HistoricalItem struct {
	Name      string `gorm:"not null"`
	Price     int
	URL       string    `gorm:"primary_index"`
	CreatedAt time.Time `gorm:"primary_index"`
}
