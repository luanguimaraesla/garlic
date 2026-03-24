// Package global exposes application-level metadata.
//
// [Version] holds the application version string, read from the XCI_APP_VERSION
// environment variable at package init time. If the variable is unset, Version
// defaults to "undefined".
package global
