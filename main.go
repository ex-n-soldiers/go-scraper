package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/PuerkitoBio/goquery"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/spf13/viper"
	"github.com/t-tiger/gorm-bulk-insert"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	var pageOption bool
	var storeOption bool
	flag.BoolVar(&pageOption, "p", false, "page option must be bool")
	flag.BoolVar(&storeOption, "s", false, "store option must be bool")
	flag.Parse()

	config, err := configure()
	if err != nil {
		panic(err)
	}

	db, err := gormConnect(config)
	defer db.Close()
	if err != nil {
		panic(err)
	}

	response, err := getResponse(config.BaseURL)
	if err != nil {
		panic(err)
	}

	items, err := getList(response, config.NotFoundMessage)
	if err != nil {
		panic(err)
	}

	if pageOption && len(items) > 0 {
		items, err = getOtherPageList(items, config, response)
		if err != nil {
			panic(err)
		}
	}

	if err := registerCurrentData(items, db); err != nil {
		panic(err)
	}

	if err := updateItemMaster(db); err != nil {
		panic(err)
	}

	if err := fetchDetailPages(db, config.DownloadBasePath); err != nil {
		panic(err)
	}

	if storeOption && len(items) > 0 {
		if err := registerCurrentData4History(items, db); err != nil {
			panic(err)
		}
	}
}

func gormConnect(config Config) (*gorm.DB, error) {
	dbHost := config.Db.Host
	dbPort := config.Db.Port
	dbName := config.Db.DbName
	dbUser := config.Db.User
	dbPassword := config.Db.Password

	db, err := gorm.Open("mysql", fmt.Sprintf("%s:%s@(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", dbUser, dbPassword, dbHost, dbPort, dbName))
	if err != nil {
		return nil, fmt.Errorf("DB connection error: %w", err)
	}

	if err := db.AutoMigrate(&ItemMaster{}, &LatestItem{}, &HistoricalItem{}).Error; err != nil {
		return nil, fmt.Errorf("DB migration error: %w", err)
	}
	return db, nil
}

func getResponse(url string) (*http.Response, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP Get request error: %w", err)
	}
	return response, nil
}

func getList(response *http.Response, notFoundMessage string) ([]Item, error) {
	body := response.Body
	requestURL := *response.Request.URL
	var items []Item

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("Get document error: %w", err)
	}

	tr := doc.Find("table tr")
	if strings.Contains(doc.Text(), notFoundMessage) || tr.Size() == 0 {
		return nil, nil
	}
	tr.Each(func(_ int, s *goquery.Selection) {
		item := Item{}
		item.Name = s.Find("td:nth-of-type(2) a").Text()
		item.Price, _ = strconv.Atoi(strings.ReplaceAll(strings.ReplaceAll(s.Find("td:nth-of-type(3)").Text(), ",", ""), "円", ""))
		itemURL, exists := s.Find("td:nth-of-type(2) a").Attr("href")
		refURL, parseErr := url.Parse(itemURL)
		if exists && parseErr == nil {
			item.URL = (*requestURL.ResolveReference(refURL)).String()
		}
		if item.Name != "" {
			items = append(items, item)
		}
	})
	return items, nil
}

func getOtherPageList(items []Item, config Config, response *http.Response) ([]Item, error) {
	page := 2
	existsPage := true
	for existsPage == true {
		u, err := url.Parse(config.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("Parse url error: %w", err)
		}
		q := u.Query()
		q.Set("page", strconv.Itoa(page))
		u.RawQuery = q.Encode()
		response, _ = getResponse(u.String())
		l, err := getList(response, config.NotFoundMessage)
		if err != nil {
			return nil, fmt.Errorf("Get list error: %w", err)
		}
		if len(l) == 0 {
			fmt.Printf("Item is not found: %s\n", u.String())
			existsPage = false
		} else {
			items = append(items, l...)
			page++
		}
	}
	return items, nil
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
	return db.Transaction(func(tx *gorm.DB) error {
		// Insert
		var newItems []LatestItem
		if err := tx.Unscoped().Joins("left join item_master on latest_items.url = item_master.url").Where("item_master.name is null").Find(&newItems).Error; err != nil {
			return fmt.Errorf("Insert error: %w", err)
		}

		var insertRecords []interface{}
		for _, newItem := range newItems {
			insertRecords = append(insertRecords, ItemMaster{Item: newItem.Item})
			fmt.Printf("Index item is created: %s\n", newItem.URL)
		}
		if err := gormbulk.BulkInsert(tx, insertRecords, 2000); err != nil {
			return fmt.Errorf("Bulk insert error: %w", err)
		}

		// Update
		var updatedItems []LatestItem
		if err := tx.Unscoped().Joins("inner join item_master on latest_items.url = item_master.url").Where("latest_items.name <> item_master.name or latest_items.price <> item_master.price or item_master.deleted_at is not null").Find(&updatedItems).Error; err != nil {
			return fmt.Errorf("Update error: %w", err)
		}
		for _, updatedItem := range updatedItems {
			fmt.Printf("Index item is updated: %s\n", updatedItem.URL)
			if err := tx.Unscoped().Model(ItemMaster{}).Where("url = ?", updatedItem.URL).Updates(map[string]interface{}{"name": updatedItem.Name, "price": updatedItem.Price, "deleted_at": nil}).Error; err != nil {
				return fmt.Errorf("Update error: %w", err)
			}
		}

		// Delete
		var deletedItems []ItemMaster
		if err := tx.Where("not exists(select 1 from latest_items li where li.url = item_master.url)").Find(&deletedItems).Error; err != nil {
			return fmt.Errorf("Delete error: %w", err)
		}
		for _, deletedItem := range deletedItems {
			fmt.Printf("Index item is deleted: %s\n", deletedItem.URL)
		}
		if err := tx.Where("not exists(select 1 from latest_items li where li.url = item_master.url)").Delete(&ItemMaster{}).Error; err != nil {
			return fmt.Errorf("Delete error: %w", err)
		}

		return nil
	})
}

func fetchDetailPages(db *gorm.DB, downloadBasePath string) error {
	var items []ItemMaster
	if err := db.Find(&items).Error; err != nil {
		return fmt.Errorf("Select error: %w", err)
	}

	for _, item := range items {
		response, err := getResponse(item.URL)
		if err != nil {
			return fmt.Errorf("Fetch detail page body error: %w", err)
		}

		currentItem, err := getDetails(response, item, downloadBasePath)
		if err != nil {
			return fmt.Errorf("Fetch detail page content error: %w", err)
		}

		if !item.equals(currentItem) {
			if err = db.Model(&currentItem).Updates(ItemMaster{
				Description:         currentItem.Description,
				ImageURL:            currentItem.ImageURL,
				ImageLastModifiedAt: currentItem.ImageLastModifiedAt,
				ImageDownloadPath:   currentItem.ImageDownloadPath,
				PdfURL:              currentItem.PdfURL,
				PdfLastModifiedAt:   currentItem.PdfLastModifiedAt,
				PdfDownloadPath:     currentItem.PdfDownloadPath}).Error; err != nil {
				return fmt.Errorf("Update item detail info error: %w", err)
			}
			fmt.Printf("Detail page is updated: %s\n", currentItem.URL)
		}
	}
	return nil
}

func getDetails(response *http.Response, item ItemMaster, downloadBasePath string) (ItemMaster, error) {
	body := response.Body
	requestURL := *response.Request.URL
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return ItemMaster{}, fmt.Errorf("Get detail page document body error %w", err)
	}

	item.Description = doc.Find("table tr:nth-of-type(2) td:nth-of-type(2)").Text()

	// Image
	href, exists := doc.Find("table tr:nth-of-type(1) td:nth-of-type(1) img").Attr("src")
	refURL, parseErr := url.Parse(href)
	if exists && parseErr == nil {
		imageURL := (*requestURL.ResolveReference(refURL)).String()
		isUpdated, currentLastModified := checkFileUpdated(imageURL, item.ImageLastModifiedAt)
		if isUpdated {
			item.ImageURL = imageURL
			item.ImageLastModifiedAt = currentLastModified
			imageDownloadPath, err := downloadFile(imageURL, filepath.Join(downloadBasePath, "img", strconv.Itoa(int(item.ID)), item.ImageFileName()))
			if err != nil {
				return ItemMaster{}, fmt.Errorf("Download image error: %w", err)
			}
			item.ImageDownloadPath = imageDownloadPath
		}
	}

	// PDF
	href, exists = doc.Find("table tr:nth-of-type(3) td:nth-of-type(2) a").Attr("href")
	refURL, parseErr = url.Parse(href)
	if exists && parseErr == nil {
		pdfURL := (*requestURL.ResolveReference(refURL)).String()
		isUpdated, currentLastModified := checkFileUpdated(pdfURL, item.PdfLastModifiedAt)
		if isUpdated {
			item.PdfURL = pdfURL
			item.PdfLastModifiedAt = currentLastModified
			pdfDownloadPath, err := downloadFile(pdfURL, filepath.Join(downloadBasePath, "pdf", strconv.Itoa(int(item.ID)), item.PdfFileName()))
			if err != nil {
				return ItemMaster{}, fmt.Errorf("Download pdf error: %w", err)
			}
			item.PdfDownloadPath = pdfDownloadPath
		}
	}

	return item, nil
}

func checkFileUpdated(fileURL string, lastModified time.Time) (isUpdated bool, currentLastModified time.Time) {
	currentLastModified, err := getLastModified(fileURL)
	if err != nil {
		return false, currentLastModified
	}

	if currentLastModified.After(lastModified) {
		return true, currentLastModified
	} else {
		return false, lastModified
	}
}

func getLastModified(fileURL string) (time.Time, error) {
	res, err := http.Head(fileURL)
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
	if os.Getenv("s3_region") == "" && os.Getenv("s3_bucket") == "" {
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
			fmt.Println("Download File:", url)
		}
		defer resp.Body.Close()

		// Write the body to file
		if _, err = io.Copy(out, resp.Body); err != nil {
			return "", fmt.Errorf("Copy file error during download file: %w", err)
		}

		downloadedPath = filepath.Join(downloadPath, filepath.Base(downloadPath))
		return downloadedPath, nil
	} else {
		// Get the file
		resp, err := http.Get(url)
		if err != nil {
			return "", fmt.Errorf("Download file error: %w", err)
		} else {
			fmt.Println("Download File:", url)
		}
		defer resp.Body.Close()

		// Create session
		ses := session.Must(session.NewSession(&aws.Config{
			S3ForcePathStyle: aws.Bool(true),
			Region:           aws.String(os.Getenv("s3_region")),
		}))

		// Save file to S3
		uploader := s3manager.NewUploader(ses)
		result, err := uploader.Upload(&s3manager.UploadInput{
			Bucket: aws.String(os.Getenv("s3_bucket")),
			Key:    aws.String(downloadPath),
			Body:   resp.Body,
		})
		if err != nil {
			return "", fmt.Errorf("Save file error: %w", err)
		}
		fmt.Println("S3 URL:", result.Location)

		return result.Location, nil
	}
}

func configure() (Config, error) {
	var config Config
	_, localConfErr := os.Stat(filepath.Join(".", "conf", "config-local.yml"))
	_, confErr := os.Stat(filepath.Join(".", "conf", "config-local.yml"))

	if localConfErr == nil || confErr == nil {
		viper.SetDefault("db.host", "localhost")
		viper.SetDefault("db.port", "3306")
		viper.SetDefault("db.dbName", "go_scraper")
		viper.SetDefault("db.user", "user")
		viper.SetDefault("db.password", "password")
		viper.SetDefault("baseURL", "http://localhost:5000/")
		currentDirectory, err := os.Getwd()
		if err != nil {
			currentDirectory = "."
		}
		viper.SetDefault("downloadBasePath", filepath.Join(currentDirectory, "work", "downloadFiles"))
		viper.SetDefault("notFoundMessage", "ページが存在しません")

		if localConfErr == nil {
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
	}

	if os.Getenv("db_host") != "" {
		config.Host = os.Getenv("db_host")
	}
	if os.Getenv("db_instance_name") != "" {
		config.InstanceName = os.Getenv("db_instance_name")
	}
	if os.Getenv("db_db_name") != "" {
		config.DbName = os.Getenv("db_db_name")
	}
	if os.Getenv("db_port") != "" {
		config.Port = os.Getenv("db_port")
	}
	if os.Getenv("db_user") != "" {
		config.User = os.Getenv("db_user")
	}
	if os.Getenv("db_password") != "" {
		config.Password = os.Getenv("db_password")
	}
	if os.Getenv("base_url") != "" {
		config.BaseURL = os.Getenv("base_url")
	}
	if os.Getenv("download_base_path") != "" {
		config.DownloadBasePath = os.Getenv("download_base_path")
	}
	if os.Getenv("not_found_message") != "" {
		config.NotFoundMessage = os.Getenv("not_found_message")
	}

	return config, nil
}

func registerCurrentData4History(items []Item, db *gorm.DB) error {
	var insertRecords []interface{}
	var histItem HistoricalItem
	for _, item := range items {
		histItem = HistoricalItem{}
		histItem.Name = item.Name
		histItem.Price = item.Price
		histItem.URL = item.URL
		insertRecords = append(insertRecords, histItem)
	}
	if err := gormbulk.BulkInsert(db, insertRecords, 2000); err != nil {
		return fmt.Errorf("Bulk insert error: %w", err)
	}
	return nil
}
