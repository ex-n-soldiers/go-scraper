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

func (i ItemMaster) equals(target ItemMaster) bool {
	return i.Description == target.Description &&
		i.ImageUrl == target.ImageUrl &&
		i.ImageLastModifiedAt == target.ImageLastModifiedAt &&
		i.ImageDownloadPath == target.ImageDownloadPath &&
		i.PDFUrl == target.PDFUrl &&
		i.PDFLastModifiedAt == target.PDFLastModifiedAt &&
		i.PDFDownloadPath == target.PDFDownloadPath
}
