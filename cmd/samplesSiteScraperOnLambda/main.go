package main

import (
	"github.com/ex-n-soldiers/go-scraper/internal/pkg"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"net/http"
)

func main() {
	lambda.Start(handleRequest)
}

func handleRequest() (events.APIGatewayProxyResponse, error) {
	if err := lambdaRun(); err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "Succeeded",
	}, nil
}

func lambdaRun() error {
	config, err := pkg.Configure()
	if err != nil {
		return err
	}

	db, err := pkg.GormConnect(config)
	if err != nil {
		return err
	}
	defer db.Close()

	if err = pkg.DbMigration(db); err != nil {
		return err
	}

	response, err := pkg.GetResponse(config.BaseURL)
	if err != nil {
		return err
	}

	items, err := pkg.GetList(response, config.NotFoundMessage)
	if err != nil {
		return err
	}

	if len(items) > 0 {
		items, err = pkg.GetOtherPageList(items, config, response)
		if err != nil {
			return err
		}
	}

	if err := pkg.RegisterCurrentData(items, db); err != nil {
		return err
	}

	if err := pkg.UpdateItemMaster(db); err != nil {
		return err
	}

	if err := pkg.FetchDetailPages(db, config.DownloadBasePath); err != nil {
		return err
	}

	return nil
}
