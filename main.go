package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/spf13/viper"
	"github.com/t-tiger/gorm-bulk-insert"
	"io"
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
	config, err := configure()
	if err != nil {
		panic(err)
	}

	db, err := gormConnect(config)
	defer db.Close()
	if err != nil {
		panic(err)
	}

	body, err := getBody(baseURL)
	if err != nil {
		panic(err)
	}

	items, err := getList(body)
	if err != nil {
		panic(err)
	}

	if err := registerCurrentData(items, db); err != nil {
		panic(err)
	}

	if err := updateItemMaster(db); err != nil {
		panic(err)
	}

	durationDays := 5
	if err := fetchDetailPages(db, durationDays); err != nil {
		panic(err)
	}
}

func gormConnect(config Config) (*gorm.DB, error) {
	var dbHost = config.Db.Host
	var dbPort = config.Db.Port
	var dbName = config.Db.DbName
	var dbUser = config.Db.User
	var dbPassword = config.Db.Password

	db, err := gorm.Open("mysql", fmt.Sprintf("%s:%s@(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", dbUser, dbPassword, dbHost, dbPort, dbName))
	if err != nil {
		return nil, fmt.Errorf("DB connection error: %w", err)
	}

	if err := db.AutoMigrate(&ItemMaster{}, &LatestItem{}).Error; err != nil {
		return nil, fmt.Errorf("DB migration error: %w", err)
	}
	return db, nil
}

func getBody(url string) (io.ReadCloser, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP Get request error: %v %s %w", res.StatusCode, res.Status, err)
	}
	return res.Body, nil
}

func getList(body io.ReadCloser) ([]Item, error) {
	var items []Item

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("Get document error: %w", err)
	}

	doc.Find("table tr").Each(func(_ int, s *goquery.Selection) {
		item := Item{}
		item.Name = s.Find("td:nth-of-type(2) a").Text()
		item.Price, _ = strconv.Atoi(strings.ReplaceAll(strings.ReplaceAll(s.Find("td:nth-of-type(3)").Text(), ",", ""), "円", ""))
		uri, exists := s.Find("td:nth-of-type(2) a").Attr("href")
		if exists {
			item.Url = baseURL + uri
		}
		if item.Name != "" {
			items = append(items, item)
		}
	})
	return items, err
}

func registerCurrentData(items []Item, db *gorm.DB) error {
	if err := db.Exec("TRUNCATE " + db.NewScope(&LatestItem{}).TableName()).Error; err != nil {
		return fmt.Errorf("Truncate table error: %w", err)
	}

	var insertRecords []interface{}
	for _, item := range items {
		insertRecords = append(insertRecords, LatestItem{Item: item})
	}
	if err := gormbulk.BulkInsert(db, insertRecords, 2000); err != nil {
		return fmt.Errorf("Bulk insert error: %w", err)
	}
	return nil
}

func updateItemMaster(db *gorm.DB) error {
	// Insert
	var newItems []LatestItem
	if err := db.Unscoped().Joins("left join item_master on latest_items.url = item_master.url").Where("item_master.name is null").Find(&newItems).Error; err != nil {
		return fmt.Errorf("Insert error: %w", err)
	}

	var insertRecords []interface{}
	for _, newItem := range newItems {
		insertRecords = append(insertRecords, ItemMaster{Item: newItem.Item})
	}
	if err := gormbulk.BulkInsert(db, insertRecords, 2000); err != nil {
		return fmt.Errorf("Bulk insert error: %w", err)
	}

	// Update
	var updatedItems []LatestItem
	if err := db.Unscoped().Joins("inner join item_master on latest_items.url = item_master.url").Where("latest_items.name <> item_master.name or latest_items.price <> item_master.price or item_master.deleted_at is not null").Find(&updatedItems).Error; err != nil {
		return fmt.Errorf("Update error: %w", err)
	}
	for _, updatedItem := range updatedItems {
		if err := db.Unscoped().Model(ItemMaster{}).Where("url = ?", updatedItem.Url).Updates(map[string]interface{}{"nam": updatedItem.Name, "price": updatedItem.Price, "deleted_at": nil}).Error; err != nil {
			return fmt.Errorf("Update error: %w", err)
		}
	}

	// Delete
	if err := db.Where("not exists(select 1 from latest_items li where li.url = item_master.url)").Delete(&ItemMaster{}).Error; err != nil {
		return fmt.Errorf("Delete error: %w", err)
	}

	return nil
}

func fetchDetailPages(db *gorm.DB, durationDays int) error {
	var items []ItemMaster
	if err := db.Where("last_checked_at < ?", time.Now().AddDate(0, 0, -durationDays)).Find(&items).Error; err != nil {
		return fmt.Errorf("Select error: %w", err)
	}

	for _, item := range items {
		body, err := getBody(item.Url)
		if err != nil {
			return fmt.Errorf("Fetch detail page body error: %w", err)
		}

		itemWithDetails, err := getDetails(body, item)
		if err != nil {
			return fmt.Errorf("Fetch detail page content error: %w", err)
		}

		if err = db.Model(&itemWithDetails).Updates(ItemMaster{
			Description: itemWithDetails.Description,
			LastCheckedAt: time.Now(),
			ImageUrl: itemWithDetails.ImageUrl,
			ImageLastModifiedAt: itemWithDetails.ImageLastModifiedAt,
			ImageDownloadPath: itemWithDetails.ImageDownloadPath,
			PDFUrl: itemWithDetails.PDFUrl,
			PDFLastModifiedAt: itemWithDetails.PDFLastModifiedAt,
			PDFDownloadPath: itemWithDetails.PDFDownloadPath}).Error; err != nil {
			return fmt.Errorf("Update item detail info error: %w", err)
		}
	}
	return nil
}

func getDetails(body io.ReadCloser, item ItemMaster) (ItemMaster, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return ItemMaster{}, fmt.Errorf("Get detail page document body error %w", err)
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
			return ItemMaster{}, fmt.Errorf("Download image error: %w", err)
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
			return ItemMaster{}, fmt.Errorf("Download pdf error: %w", err)
		}
		item.PDFDownloadPath = pdfDownloadPath
	}

	return item, nil
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
		return time.Time{}, fmt.Errorf("HTTP HEAD request error: %w", err)
	}
	lastModified, err := time.Parse("Mon, 02 Jan 2006 15:04:05 MST", res.Header.Get("Last-Modified"))
	if err != nil {
		return time.Time{}, fmt.Errorf("Get last-modified attribute error: %w", err)
	}
	return lastModified, nil
}

func downloadFile(url string, downloadPath string) (downloadedPath string, err error) {
	// Create base directory
	err = os.MkdirAll(filepath.Dir(downloadPath), 0777)
	if err != nil {
		return "", fmt.Errorf("Mkdir error during download file: %w", err)
	}

	// Create the file
	out, err := os.Create(downloadPath)
	if err != nil {
		return "", fmt.Errorf("Create file error during download file: %w", err)
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("Download file error: %w", err)
	} else {
		fmt.Println("Download File: ", url)
	}
	defer resp.Body.Close()

	// Write the body to file
	if _, err = io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("Copy file error during download file: %w", err)
	}

	downloadedPath = downloadPath + filepath.Base(downloadPath)
	return downloadedPath, nil
}

func configure() (Config, error) {
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
		return Config{}, fmt.Errorf("Read config file error: %w", err)
	}

	if err := viper.Unmarshal(&config); err != nil {
		return Config{}, fmt.Errorf("Unmarshal config file error: %w", err)
	}

	return config, nil
}
