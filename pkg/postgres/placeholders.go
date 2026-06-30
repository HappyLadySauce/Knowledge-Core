package postgres

import (
	"strconv"
	"strings"
)

// Placeholders returns comma-separated PostgreSQL placeholders, starting at start.
// Placeholders 返回从 start 开始的 PostgreSQL 占位符列表。
func Placeholders(start, count int) string {
	values := make([]string, 0, count)
	for i := 0; i < count; i++ {
		values = append(values, "$"+strconv.Itoa(start+i))
	}
	return strings.Join(values, ",")
}
