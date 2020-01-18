package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/t-tiger/gorm-bulk-insert"
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

	indexItems, err := getList(body, baseURL)
	if err != nil {
		panic(err)
	}

	err = registerCurrentData(indexItems, db)
	if err != nil {
		panic(err)
	}

	err = updateItemMaster(db)
	if err != nil {
		panic(err)
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

	db.AutoMigrate(&ItemMaster{}, &LatestItem{})
	return db
}

func getBody(url string) (io.ReadCloser, error) {
	res, err := http.Get(url)
	if err != nil {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}
	return res.Body, err
}

func getList(body io.ReadCloser, baseURL string) ([]Item, error) {
	var itemList []Item

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("table tr").Each(func(_ int, s *goquery.Selection) {
		item := Item{}
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

func registerCurrentData(items []Item, db *gorm.DB) error {
	db.Exec("TRUNCATE " + db.NewScope(&LatestItem{}).TableName())

	var insertRecords []interface{}
	for _, item := range items {
		insertRecords = append(insertRecords, LatestItem{Item: item})
	}
	err := gormbulk.BulkInsert(db, insertRecords, 2000)
	return err
}

func updateItemMaster(db *gorm.DB) error {
	// Insert
	var newItems []LatestItem
	err := db.Unscoped().Joins("left join item_master on latest_items.url = item_master.url").Where("item_master.name is null").Find(&newItems).Error
	if err != nil {
		return err
	}

	var insertRecords []interface{}
	for _, newItem := range newItems {
		insertRecords = append(insertRecords, ItemMaster{Item: newItem.Item})
	}
	err = gormbulk.BulkInsert(db, insertRecords, 2000)
	if err != nil {
		return err
	}

	// Update
	var updatedItems []LatestItem
	err = db.Unscoped().Joins("inner join item_master on latest_items.url = item_master.url").Where("latest_items.name <> item_master.name or latest_items.price <> item_master.price or item_master.deleted_at is not null").Find(&updatedItems).Error
	if err != nil {
		return err
	}
	for _, updatedItem := range updatedItems {
		err := db.Unscoped().Model(ItemMaster{}).Where("url = ?", updatedItem.Url).Updates(map[string]interface{}{"nam": updatedItem.Name, "price": updatedItem.Price, "deleted_at": nil}).Error
		if err != nil {
			return err
		}
	}

	// Delete
	err = db.Where("not exists(select 1 from latest_items li where li.url = item_master.url)").Delete(&ItemMaster{}).Error
	return err
}
