package manualtx

import (
	"database/sql"

	"github.com/luanguimaraesla/garlic/database"
)

func invalid(d *database.DB) error {
	tx, err := d.Begin() // want "\\[G3.01\\]"
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil { // want "\\[G3.01\\]"
		return err
	}
	return tx.Rollback() // want "\\[G3.01\\]"
}

func invalidStdlib(db *sql.DB) error {
	tx, err := db.Begin() // want "\\[G3.01\\]"
	if err != nil {
		return err
	}
	return tx.Rollback() // want "\\[G3.01\\]"
}

type queue struct{}

func (queue) Begin() {}

func acceptable(q queue) {
	_ = database.NewStorer
	q.Begin()
}
