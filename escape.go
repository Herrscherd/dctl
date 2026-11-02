package dctl

import "net/url"

func seg(s string) string { return url.PathEscape(s) }
