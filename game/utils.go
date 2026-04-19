package game

import (
	"fmt"
	"time"
)

func intPow10(n int) int {
	result := 1
	for range n {
		result *= 10
	}
	return result
}

func formatAge(d time.Duration) string {
	seconds := max(int(d.Seconds()), 0)
	m := seconds / 60
	s := seconds % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
