package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func getResponse(url string) (*http.Response, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http get request error: %w", err)
	}
	return response, nil
}

func getList(response *http.Response) ([]Item, error) {
	// レスポンスボディを取得
	body := response.Body

	// レスポンスに含まれているリクエスト情報からリクエスト先のURLを取得
	// 相対URLで書かれた詳細ページへのリンクを絶対URLで構成するために取得
	requestURL := *response.Request.URL

	var items []Item

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("get document error: %w", err)
	}

	// Find関数の引数でselectorを設定
	tr := doc.Find("table tr")
	notFoundMessage := "ページが存在しません"
	if strings.Contains(doc.Text(), notFoundMessage) || tr.Size() == 0 {
		return nil, nil
	}

	tr.Each(func(_ int, s *goquery.Selection) {
		item := Item{}

		// Find関数を使用して商品の各要素を取得
		item.Name = s.Find("td:nth-of-type(2) a").Text()
		item.Price, _ = strconv.Atoi(strings.ReplaceAll(strings.ReplaceAll(s.Find("td:nth-of-type(3)").Text(), ",", ""), "円", ""))
		itemURL, exists := s.Find("td:nth-of-type(2) a").Attr("href")
		refURL, parseErr := url.Parse(itemURL)

		if exists && parseErr == nil {
			// requestURLとrefURLを結合して絶対URLを取得
			item.Url = (*requestURL.ResolveReference(refURL)).String()
		}

		if item.Name != "" {
			items = append(items, item)
		}
	})

	return items, nil
}

func fetchDetailPages(items []ItemMaster, downloadBasePath string) ([]ItemMaster, error) {
	parsePage := func(response *http.Response, item ItemMaster) (ItemMaster, error) {
		body := response.Body
		requestURL := *response.Request.URL
		doc, err := goquery.NewDocumentFromReader(body)
		if err != nil {
			return ItemMaster{}, fmt.Errorf("get detail page document body error %w", err)
		}

		item.Description = doc.Find("table tr:nth-of-type(2) td:nth-of-type(2)").Text()

		// Image
		href, exists := doc.Find("table tr:nth-of-type(1) td:nth-of-type(1) img").Attr("src")
		refURL, parseErr := url.Parse(href)
		if exists && parseErr == nil {
			imageURL := (*requestURL.ResolveReference(refURL)).String()
			isUpdated, currentLastModified := checkFileUpdated(imageURL, item.ImageLastModifiedAt)
			if isUpdated {
				item.ImageUrl = imageURL
				item.ImageLastModifiedAt = currentLastModified
				imageDownloadPath, err := downloadFile(imageURL, filepath.Join(downloadBasePath, "img", strconv.Itoa(int(item.ID)), item.ImageFileName()))
				if err != nil {
					return ItemMaster{}, fmt.Errorf("download image error: %w", err)
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
				item.PdfUrl = pdfURL
				item.PdfLastModifiedAt = currentLastModified
				pdfDownloadPath, err := downloadFile(pdfURL, filepath.Join(downloadBasePath, "pdf", strconv.Itoa(int(item.ID)), item.PdfFileName()))
				if err != nil {
					return ItemMaster{}, fmt.Errorf("download pdf error: %w", err)
				}
				item.PdfDownloadPath = pdfDownloadPath
			}
		}

		return item, nil
	}

	var updatedItems []ItemMaster

	for _, item := range items {
		response, err := getResponse(item.Url)
		if err != nil {
			return nil, fmt.Errorf("fetch detail page body error: %w", err)
		}

		currentItem, err := parsePage(response, item)
		if err != nil {
			return nil, fmt.Errorf("fetch detail page content error: %w", err)
		}

		if !item.equals(currentItem) {
			updatedItems = append(updatedItems, currentItem)
		}
	}

	return updatedItems, nil
}

func checkFileUpdated(fileURL string, lastModified time.Time) (isUpdated bool, currentLastModified time.Time) {
	getLastModified := func(fileURL string) (time.Time, error) {
		res, err := http.Head(fileURL)
		if err != nil {
			return time.Time{}, fmt.Errorf("http head request error: %w", err)
		}
		lastModified, err := time.Parse("Mon, 02 Jan 2006 15:04:05 MST", res.Header.Get("Last-Modified"))
		if err != nil {
			return time.Time{}, fmt.Errorf("get last-modified attribute error: %w", err)
		}
		return lastModified, nil
	}

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
