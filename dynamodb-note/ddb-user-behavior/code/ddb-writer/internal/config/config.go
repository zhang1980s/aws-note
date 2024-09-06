package config

import (
	"flag"
	"log"
	"os"
)

type Config struct {
	TableName        string
	TotalRecords     int
	RecordsPerSecond int
}

func Load() *Config {
	cfg := &Config{}
	flag.StringVar(&cfg.TableName, "t", os.Getenv("DDB_TABLE_NAME"), "DynamoDB table name")
	flag.IntVar(&cfg.TotalRecords, "n", 100000, "Total number of records to write")
	flag.IntVar(&cfg.RecordsPerSecond, "r", 100, "Number of records to write per second (max 2000000)")
	flag.Parse()

	if cfg.RecordsPerSecond > 2000000 {
		log.Fatal("Records per second cannot exceed 2000000")
	}

	return cfg
}
