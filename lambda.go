package main

import "github.com/aws/aws-lambda-go/lambda"

// Lambda用のバイナリファイルを作成する場合は関数名をmainに変更
func lambdaMain() {
	lambda.Start(handleRequest)
}

func handleRequest() (string, error) {
	if err := lambdaRun(); err != nil {
		return "", err
	}
	return "Succeeded!", nil
}

func lambdaRun() error {
	config, err := configure()
	if err != nil {
		return err
	}

	db, err := gormConnect(config)
	if err != nil {
		return err
	}
	defer db.Close()

	err = dbMigration(db)
	if err != nil {
		panic(err)
	}

	response, err := getResponse(config.BaseURL)
	if err != nil {
		return err
	}

	items, err := getList(response, config.NotFoundMessage)
	if err != nil {
		return err
	}

	if len(items) > 0 {
		items, err = getOtherPageList(items, config, response)
		if err != nil {
			return err
		}
	}

	if err := registerCurrentData(items, db); err != nil {
		return err
	}

	if err := updateItemMaster(db); err != nil {
		return err
	}

	if err := fetchDetailPages(db, config.DownloadBasePath); err != nil {
		return err
	}

	return nil
}
