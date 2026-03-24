package database_test

import (
	"context"

	"github.com/luanguimaraesla/garlic/database"
)

func ExampleNew() {
	db := database.New(&database.Config{
		Host:     "localhost",
		Port:     5432,
		Database: "myapp",
		Username: "postgres",
		Password: "secret",
		SSLMode:  database.SSLModeDisable,
	})
	if err := db.Connect(); err != nil {
		panic(err)
	}
}

func ExampleStorer_Transaction() {
	// Assuming db is a connected *database.Database:
	var db database.Store

	storer := database.NewStorer(db)
	err := storer.Transaction(context.Background(), func(txCtx context.Context) error {
		// All operations within this function share the same transaction.
		// On success the transaction is committed; on error or panic it is
		// rolled back automatically.
		return nil
	})
	_ = err
}
