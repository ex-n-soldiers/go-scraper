package main

type Config struct {
	Db struct {
		Host string
		InstanceName string
		DbName string
		Port string
		User string
		Password string
	}
}
