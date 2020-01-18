package main

import "github.com/jinzhu/gorm"

type IndexItem struct {
	gorm.Model
	Name string `gorm:"not null"`
	Price int
	Url string
}

func (orgItem *IndexItem) equals(targetItem IndexItem) bool {
	return orgItem.Name == targetItem.Name && orgItem.Price == targetItem.Price && orgItem.Url == targetItem.Url
}
