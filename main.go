package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func main() {
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
		fmt.Println(item)
	}
}

func getBody(url string) (io.ReadCloser, error) {
	res, err := http.Get(url)
	if err != nil {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}
	return res.Body, err
}

func getList(body io.ReadCloser, baseURL string) ([]indexItem, error) {
	var itemList []indexItem

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("table tr").Each(func(_ int, s *goquery.Selection) {
		item := indexItem{}
		item.name = s.Find("td:nth-of-type(2) a").Text()
		item.price, _ = strconv.Atoi(strings.ReplaceAll(strings.ReplaceAll(s.Find("td:nth-of-type(3)").Text(), ",", ""), "円", ""))
		uri, _ := s.Find("td:nth-of-type(2) a").Attr("href")
		item.url = baseURL + uri
		if item.name != "" {
			itemList = append(itemList, item)
		}
	})
	return itemList, err
}
