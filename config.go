package main

type Config struct {
	Db
	BaseURL          string
	DownloadBasePath string
	NotFoundMessage  string
}

type Db struct {
	Host         string
	InstanceName string
	DbName       string
	Port         string
	User         string
	Password     string
}
