package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type modelStats struct {
	todayCount int
	totalCount int
	totalLatMs int64
	latCount   int // success entries only
}

func runStats() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stats: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}
	logPath := filepath.Join(home, ".claude", "logs", "oh-my-bridge.log")

	f, err := os.Open(logPath) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("로그 없음")
			return
		}
		fmt.Fprintf(os.Stderr, "stats: %v\n", err)
		os.Exit(1)
	}

	todayDate := time.Now().UTC().Format("2006-01-02")

	// preserve insertion order
	order := []string{}
	seen := map[string]bool{}
	stats := map[string]*modelStats{}

	getOrCreate := func(key string) *modelStats {
		if !seen[key] {
			seen[key] = true
			order = append(order, key)
			stats[key] = &modelStats{}
		}
		return stats[key]
	}

	var malformedCount int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e logEntry
		if err := json.Unmarshal(line, &e); err != nil {
			malformedCount++
			continue
		}

		key := e.Model
		if e.Status == "claude" {
			key = "claude (direct)"
		}

		s := getOrCreate(key)
		s.totalCount++

		ts, err := time.Parse(time.RFC3339Nano, e.Timestamp)
		if err == nil && ts.UTC().Format("2006-01-02") == todayDate {
			s.todayCount++
		}

		if e.Status == "success" && e.LatencyMs > 0 {
			s.totalLatMs += e.LatencyMs
			s.latCount++
		}
	}
	if malformedCount > 0 {
		fmt.Fprintf(os.Stderr, "stats: skipped %d malformed log line(s)\n", malformedCount)
	}
	f.Close() //nolint:errcheck,gosec
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "stats: reading log: %v\n", err)
		os.Exit(1)
	}

	if len(order) == 0 {
		fmt.Println("로그 없음")
		return
	}

	// compute totals for delegated (non-claude-direct)
	var delegatedToday, delegatedTotal int
	for key, s := range stats {
		if key != "claude (direct)" {
			delegatedToday += s.todayCount
			delegatedTotal += s.totalCount
		}
	}

	sep := "─────────────────────────────────────────"

	fmt.Println("oh-my-bridge stats")
	fmt.Println()
	fmt.Println("모델별 호출 수  (오늘 / 전체)")
	fmt.Println(sep)

	for _, key := range order {
		s := stats[key]
		counts := fmt.Sprintf("%d / %d", s.todayCount, s.totalCount)
		var latStr string
		if key == "claude (direct)" || s.latCount == 0 {
			latStr = "—"
		} else {
			avgSec := float64(s.totalLatMs) / float64(s.latCount) / 1000.0
			latStr = fmt.Sprintf("평균 응답 %.1fs", avgSec)
		}
		fmt.Printf("%-22s  %-10s  %s\n", key, counts, latStr)
	}

	fmt.Println(sep)
	fmt.Printf("%-22s  %d / %d\n", "총 위임", delegatedToday, delegatedTotal)
}
