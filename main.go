package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func main() {
	db := gormConnect()
	defer db.Close()

	baseURL := "http://localhost:3000/"

	body, err := getBody(baseURL)
	if err != nil {
		panic(err)
	}

	indexItem, err := getList(body, baseURL)
	if err != nil {
		panic(err)
	}

	for _, item := range indexItem {
		var oldItem IndexItem
		err := db.First(&oldItem, "url = ?", item.Url).Error

		if err != nil {
			// insert
			log.Println(fmt.Sprintf("New record: %s(%s)", item.Name, item.Url))
			err := db.Create(&item).Error
			if err != nil {
				log.Println("Insert record error has occurred: ", err)
			}
		} else if !item.equals(oldItem) {
			// update
			log.Println(fmt.Sprintf("Update record: %s(%s)", item.Name, item.Url))
			err = db.Model(&oldItem).Updates(item).Error
			if err != nil {
				log.Println("update record error has occurred: ", err)
			}
		} else {
			// no change
			log.Println(fmt.Sprintf("No change: %s(%s)", item.Name, item.Url))
		}
	}
}

func gormConnect() *gorm.DB {
	var dbHost = "localhost"
	var dbPort = "3306"
	var dbName = "go-scraper-dev"
	var dbUser = "root"
	var dbPassword = "root"

	db, err := gorm.Open("mysql", fmt.Sprintf("%s:%s@(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", dbUser, dbPassword, dbHost, dbPort, dbName))
	if err != nil {
		panic(err.Error())
	}

	db.AutoMigrate(&IndexItem{})
	return db
}

func getBody(url string) (io.ReadCloser, error) {
	res, err := http.Get(url)
	if err != nil {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}
	return res.Body, err
}

func getList(body io.ReadCloser, baseURL string) ([]IndexItem, error) {
	var itemList []IndexItem

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("table tr").Each(func(_ int, s *goquery.Selection) {
		item := IndexItem{}
		item.Name = s.Find("td:nth-of-type(2) a").Text()
		item.Price, _ = strconv.Atoi(strings.ReplaceAll(strings.ReplaceAll(s.Find("td:nth-of-type(3)").Text(), ",", ""), "円", ""))
		uri, _ := s.Find("td:nth-of-type(2) a").Attr("href")
		item.Url = baseURL + uri
		if item.Name != "" {
			itemList = append(itemList, item)
		}
	})
	return itemList, err
}
