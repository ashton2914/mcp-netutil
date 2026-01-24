package cache

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
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
	DB, err = sql.Open("sqlite", dbPath)
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
	query := `CREATE TABLE IF NOT EXISTS records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT,
		tool_name TEXT,
		mcp_output TEXT
	)`

	if _, err := DB.Exec(query); err != nil {
		return err
	}
	return nil
}

// SaveRecord saves a tool execution record
func SaveRecord(toolName, output string) error {
	if DB == nil {
		return nil
	}
	// mcp_output is the raw JSON string
	_, err := DB.Exec("INSERT INTO records (timestamp, tool_name, mcp_output) VALUES (?, ?, ?)", LocalTimeNow(), toolName, output)
	return err
}

// QueryRecords retrieves records based on criteria
func QueryRecords(toolName, startTime, endTime string) ([]map[string]interface{}, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := "SELECT timestamp, tool_name, mcp_output FROM records WHERE 1=1"
	var args []interface{}

	if toolName != "" {
		query += " AND tool_name = ?"
		args = append(args, toolName)
	}
	if startTime != "" {
		query += " AND timestamp >= ?"
		args = append(args, startTime)
	}
	if endTime != "" {
		query += " AND timestamp <= ?"
		args = append(args, endTime)
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
