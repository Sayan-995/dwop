package repository

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	storage_go "github.com/supabase-community/storage-go"
	supabase "github.com/supabase-community/supabase-go"
)

var (
	DB            *supabase.Client
	StorageClient *storage_go.Client
)

func init() {
	godotenv.Load()
	err := NewDB()
	if err != nil {
		log.Fatalf("error setting up the DB connection: %v", err)
	}
	StorageClient = storage_go.NewClient(fmt.Sprintf("%v/storage/v1", os.Getenv("SUPABASE_PROJECT_URL")), os.Getenv("SUPABASE_SERVICE_KEY"), nil)
}

func NewDB() error {
	godotenv.Load()
	url := os.Getenv("SUPABASE_PROJECT_URL")
	key := os.Getenv("SUPABASE_SERVICE_KEY")

	fmt.Println(url)

	client, err := supabase.NewClient(url, key, nil)

	if err != nil {
		return err
	}

	DB = client
	return nil
}
