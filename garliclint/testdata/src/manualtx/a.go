package manualtx

import "github.com/luanguimaraesla/garlic/database"

type db struct{}

func (db) Begin() {}

func invalid(d db) {
	_ = database.NewStorer
	d.Begin() // want "\\[G3.01\\]"
}
