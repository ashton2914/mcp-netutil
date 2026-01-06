package cache

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

// LocalTimeNow returns the current local time formatted as YYYYMMDDhhmmss
func LocalTimeNow() string {
	return time.Now().Local().Format("20060102150405")
}

// Init initializes the SQLite database at the specified directory
func Init(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	dbPath := filepath.Join(dir, "cache.db")
	var err error
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err := createTables(); err != nil {
		DB.Close()
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}

func createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS latency_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TEXT,
			target TEXT,
			mode TEXT,
			result TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS traceroute_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TEXT,
			target TEXT,
			result TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS system_stats_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TEXT,
			result TEXT
		)`,
	}

	for _, query := range queries {
		if _, err := DB.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

// RecordLatency saves a latency check result
func RecordLatency(target, mode, result string) error {
	if DB == nil {
		return nil
	}
	_, err := DB.Exec("INSERT INTO latency_records (timestamp, target, mode, result) VALUES (?, ?, ?, ?)", LocalTimeNow(), target, mode, result)
	return err
}

// RecordTraceroute saves a traceroute result
func RecordTraceroute(target, result string) error {
	if DB == nil {
		return nil
	}
	_, err := DB.Exec("INSERT INTO traceroute_records (timestamp, target, result) VALUES (?, ?, ?)", LocalTimeNow(), target, result)
	return err
}

// RecordSystemStats saves system statistics
func RecordSystemStats(result string) error {
	if DB == nil {
		return nil
	}
	_, err := DB.Exec("INSERT INTO system_stats_records (timestamp, result) VALUES (?, ?)", LocalTimeNow(), result)
	return err
}

// QueryRecords retrieves records based on criteria
func QueryRecords(tool string, startTime, endTime, target string) ([]map[string]interface{}, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var tableName string
	switch tool {
	case "latency":
		tableName = "latency_records"
	case "traceroute":
		tableName = "traceroute_records"
	case "system_stats":
		tableName = "system_stats_records"
	default:
		return nil, fmt.Errorf("unknown tool: %s", tool)
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE 1=1", tableName)
	var args []interface{}

	if startTime != "" {
		query += " AND timestamp >= ?"
		args = append(args, startTime)
	}
	if endTime != "" {
		query += " AND timestamp <= ?"
		args = append(args, endTime)
	}
	if target != "" && tool != "system_stats" {
		query += " AND target LIKE ?"
		args = append(args, "%"+target+"%")
	}

	query += " ORDER BY timestamp DESC"

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		m := make(map[string]interface{})
		for i, colName := range cols {
			val := columns[i]
			b, ok := val.([]byte)
			if ok {
				m[colName] = string(b)
			} else {
				m[colName] = val
			}
		}
		results = append(results, m)
	}

	return results, nil
}
