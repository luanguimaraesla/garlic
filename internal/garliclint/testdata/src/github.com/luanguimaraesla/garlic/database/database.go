package database

type DB struct{}
type Tx struct{}
type Storer struct{}

func NewStorer(...any) *Storer           { return &Storer{} }
func (*DB) BeginTx(...any) (*Tx, error)  { return &Tx{}, nil }
func (*DB) BeginTxx(...any) (*Tx, error) { return &Tx{}, nil }
func (*DB) Begin(...any) (*Tx, error)    { return &Tx{}, nil }
func (*Tx) Commit() error                { return nil }
func (*Tx) Rollback() error              { return nil }
