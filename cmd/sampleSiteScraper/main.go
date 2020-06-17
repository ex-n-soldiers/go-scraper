package main

import (
	"flag"
	"github.com/ex-n-soldiers/go-scraper/internal/pkg"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

// Lambda用のバイナリファイルを作成する場合は関数名を任意の名前に変更
func main() {
	var pageOption bool
	var storeOption bool
	flag.BoolVar(&pageOption, "p", false, "page option must be bool")
	flag.BoolVar(&storeOption, "s", false, "store option must be bool")
	flag.Parse()

	config, err := pkg.Configure()
	if err != nil {
		panic(err)
	}

	db, err := pkg.GormConnect(config)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = pkg.DbMigration(db)
	if err != nil {
		panic(err)
	}

	response, err := pkg.GetResponse(config.BaseURL)
	if err != nil {
		panic(err)
	}

	items, err := pkg.GetList(response, config.NotFoundMessage)
	if err != nil {
		panic(err)
	}

	if pageOption && len(items) > 0 {
		items, err = pkg.GetOtherPageList(items, config, response)
		if err != nil {
			panic(err)
		}
	}

	if err := pkg.RegisterCurrentData(items, db); err != nil {
		panic(err)
	}

	if err := pkg.UpdateItemMaster(db); err != nil {
		panic(err)
	}

	if err := pkg.FetchDetailPages(db, config.DownloadBasePath); err != nil {
		panic(err)
	}

	if storeOption && len(items) > 0 {
		if err := pkg.RegisterCurrentData4History(items, db); err != nil {
			panic(err)
		}
	}
}
