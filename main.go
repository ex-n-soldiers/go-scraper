package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/spf13/viper"
	"github.com/t-tiger/gorm-bulk-insert"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var baseURL = "http://localhost:3000/"
var currentDirectory, _ = os.Getwd()
var downloadBasePath = filepath.Join(currentDirectory, "work", "downloadFiles")

func main() {
	config := configure()

	db := gormConnect(config)
	defer db.Close()

	body, err := getBody(baseURL)
	if err != nil {
		panic(err)
	}

	items, err := getList(body)
	if err != nil {
		panic(err)
	}

	err = registerCurrentData(items, db)
	if err != nil {
		panic(err)
	}

	err = updateItemMaster(db)
	if err != nil {
		panic(err)
	}

	durationDays := 5
	err = fetchDetailPages(db, durationDays)
	if err != nil {
		panic(err)
	}
}

func gormConnect(config Config) *gorm.DB {
	var dbHost = config.Db.Host
	var dbPort = config.Db.Port
	var dbName = config.Db.DbName
	var dbUser = config.Db.User
	var dbPassword = config.Db.Password

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

func getList(body io.ReadCloser) ([]Item, error) {
	var items []Item

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
			items = append(items, item)
		}
	})
	return items, err
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

func fetchDetailPages(db *gorm.DB, durationDays int) error {
	var items []ItemMaster
	err := db.Where("last_checked_at < ?", time.Now().AddDate(0, 0, -durationDays)).Find(&items).Error
	if err != nil {
		return err
	}

	for _, item := range items {
		body, err := getBody(item.Url)
		if err != nil {
			return err
		}

		itemWithDetails, err := getDetails(body, item)
		if err != nil {
			return err
		}

		err = db.Model(&itemWithDetails).Updates(ItemMaster{
			Description: itemWithDetails.Description,
			LastCheckedAt: time.Now(),
			ImageUrl: itemWithDetails.ImageUrl,
			ImageLastModifiedAt: itemWithDetails.ImageLastModifiedAt,
			ImageDownloadPath: itemWithDetails.ImageDownloadPath,
			PDFUrl: itemWithDetails.PDFUrl,
			PDFLastModifiedAt: itemWithDetails.PDFLastModifiedAt,
			PDFDownloadPath: itemWithDetails.PDFDownloadPath}).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func getDetails(body io.ReadCloser, item ItemMaster) (ItemMaster, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		log.Fatal(err)
	}

	item.Description = doc.Find("table tr:nth-of-type(2) td:nth-of-type(2)").Text()

	// Image
	href, exists := doc.Find("table tr:nth-of-type(1) td:nth-of-type(1) img").Attr("src")
	imageUrl := baseURL + href
	isUpdated, currentLastModified := checkFileUpdated(imageUrl, item.ImageLastModifiedAt)
	if exists && isUpdated {
		item.ImageUrl = imageUrl
		item.ImageLastModifiedAt = currentLastModified
		imageDownloadPath, err := downloadFile(imageUrl, filepath.Join(downloadBasePath, "img", strconv.Itoa(int(item.ID)), item.ImageFileName()))
		if err != nil {
			return item, err
		}
		item.ImageDownloadPath = imageDownloadPath
	}

	// PDF
	href, exists = doc.Find("table tr:nth-of-type(3) td:nth-of-type(2) a").Attr("href")
	pdfUrl := baseURL + href
	isUpdated, currentLastModified = checkFileUpdated(pdfUrl, item.PDFLastModifiedAt)
	if exists && isUpdated {
		item.PDFUrl = pdfUrl
		item.PDFLastModifiedAt = currentLastModified
		pdfDownloadPath, err := downloadFile(pdfUrl, filepath.Join(downloadBasePath, "pdf", strconv.Itoa(int(item.ID)), item.PDFFileName()))
		if err != nil {
			return item, err
		}
		item.PDFDownloadPath = pdfDownloadPath
	}

	return item, err
}

func checkFileUpdated(fileUrl string, lastModified time.Time) (isUpdated bool, currentLastModified time.Time) {
	currentLastModified, err := getLastModified(fileUrl)
	if err != nil {
		return false, currentLastModified
	}

	if currentLastModified.After(lastModified) {
		return true, currentLastModified
	} else {
		return false, lastModified
	}
}

func getLastModified(fileUrl string) (time.Time, error) {
	res, err := http.Head(fileUrl)
	if err != nil {
		return time.Unix(0, 0), err
	}
	lastModified, err := time.Parse("Mon, 02 Jan 2006 15:04:05 MST", res.Header.Get("Last-Modified"))
	return lastModified, err
}

func downloadFile(url string, downloadPath string) (downloadedPath string, err error) {
	// Create base directory
	err = os.MkdirAll(filepath.Dir(downloadPath), 0777)
	if err != nil {
		return "", err
	}

	// Create the file
	out, err := os.Create(downloadPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	// Get the data
	fmt.Println("Download File: " + url)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	downloadedPath = downloadPath + filepath.Base(downloadPath)
	return downloadedPath, nil
}

func configure() Config {
	var config Config
	viper.SetDefault("db.host", "localhost")
	viper.SetDefault("db.port", "3306")
	viper.SetDefault("db.dbName", "go-scraper")
	viper.SetDefault("db.user", "user")
	viper.SetDefault("db.password", "password")
	_, err := os.Stat(filepath.Join(".", "conf", "config-local.yml"))
	if err == nil {
		viper.SetConfigName("config-local")
	} else {
		viper.SetConfigName("config")
	}
	viper.SetConfigType("yml")
	viper.AddConfigPath(filepath.Join(".", "conf"))
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Read config file error: ", err)
		os.Exit(1)
	}

	if err := viper.Unmarshal(&config); err != nil {
		fmt.Println("Unmarshal config file error: ", err)
		os.Exit(1)
	}

	return config
}
