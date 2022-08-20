package main

func main() {
	conf, err := loadConfig()

	db, err := gormConnect(conf)
	if err != nil {
		panic(err)
	}

	err = dbMigration(db)
	if err != nil {
		panic(err)
	}

	response, err := getResponse(conf.BaseUrl)
	if err != nil {
		panic(err)
	}

	items, err := getList(response)
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

	var updateChkItems []ItemMaster
	updateChkItems, err = getItemMasters(db)
	if err != nil {
		panic(err)
	}

	var updatedItems []ItemMaster
	updatedItems, err = fetchDetailPages(updateChkItems, conf.DownloadBasePath)
	if err != nil {
		panic(err)
	}

	if err = registerDetails(db, updatedItems); err != nil {
		panic(err)
	}
}
