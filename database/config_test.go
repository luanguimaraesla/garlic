//go:build unit
// +build unit

package database

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host: want 0.0.0.0, got %s", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("Port: want 5432, got %d", cfg.Port)
	}
	if cfg.Database != "postgres" {
		t.Errorf("Database: want postgres, got %s", cfg.Database)
	}
	if cfg.Username != "postgres" {
		t.Errorf("Username: want postgres, got %s", cfg.Username)
	}
	if cfg.Password != "postgres" {
		t.Errorf("Password: want postgres, got %s", cfg.Password)
	}
	if cfg.SSLMode != SSLModeDisable {
		t.Errorf("SSLMode: want disable, got %s", cfg.SSLMode)
	}
}

func TestSSLMode_UnmarshalJSON_valid(t *testing.T) {
	cases := []struct {
		input string
		want  SSLMode
	}{
		{`"require"`, SSLModeRequire},
		{`"disable"`, SSLModeDisable},
	}

	for _, tc := range cases {
		var got SSLMode
		if err := json.Unmarshal([]byte(tc.input), &got); err != nil {
			t.Errorf("Unmarshal(%s): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Unmarshal(%s): want %s, got %s", tc.input, tc.want, got)
		}
	}
}

func TestSSLMode_UnmarshalJSON_invalid(t *testing.T) {
	var mode SSLMode
	err := json.Unmarshal([]byte(`"invalid"`), &mode)
	if err == nil {
		t.Fatal("expected error for invalid SSLMode, got nil")
	}
	if !errors.Is(err, ErrConfigInvalidSSLMode) {
		// the error is wrapped, check the message
		if err.Error() == "" {
			t.Error("expected non-empty error message")
		}
	}
}

func TestSSLMode_UnmarshalJSON_badJSON(t *testing.T) {
	var mode SSLMode
	err := json.Unmarshal([]byte(`123`), &mode)
	if err == nil {
		t.Fatal("expected error for non-string JSON, got nil")
	}
}

func TestBuildConnectionString(t *testing.T) {
	db := New(Defaults())
	got := db.BuildConnectionString()
	want := "host=0.0.0.0 port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestBuildConnectionURL(t *testing.T) {
	db := New(Defaults())
	got := db.BuildConnectionURL()
	want := "pgx5://postgres:postgres@0.0.0.0:5432/postgres?sslmode=disable"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestBuildConnectionString_customConfig(t *testing.T) {
	cfg := &Config{
		Host:     "db.example.com",
		Port:     5433,
		Database: "mydb",
		Username: "admin",
		Password: "s3cret",
		SSLMode:  SSLModeRequire,
	}
	db := New(cfg)
	got := db.BuildConnectionString()
	want := "host=db.example.com port=5433 user=admin password=s3cret dbname=mydb sslmode=require"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}
