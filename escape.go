package dctl

import (
	"net/url"
	"strings"
)

func seg(s string) string {
	escaped := url.PathEscape(s)
	if escaped == "." || escaped == ".." {
		return strings.ReplaceAll(escaped, ".", "%2E")
	}
	return escaped
}
