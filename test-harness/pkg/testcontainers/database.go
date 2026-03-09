package testcontainers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	_ "github.com/mattn/go-sqlite3"
)

// database provides fresh sqlite instances for testing
type Database struct {
	logger  *log.Logger
	path    string
	db      *sql.DB
	tempDir string
}

// databaseoptions for creating a database
type DatabaseOptions struct {
	TempDir    string
	Driver     string
	InitScript string
}

// newdatabase creates a new fresh sqlite database
func NewDatabase(ctx context.Context, logger *log.Logger, opts DatabaseOptions) (*Database, error) {
	if logger == nil {
		logger = log.New(os.Stderr)
	}

	// create temp directory if not provided
	tempDir := opts.TempDir
	if tempDir == "" {
		tempDir = os.TempDir()
	}

	// ensure temp directory exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// create unique database file
	dbPath := filepath.Join(tempDir, fmt.Sprintf("ws-test-%d.db", os.Getpid()))

	// remove if exists (fresh start)
	os.Remove(dbPath)

	db := &Database{
		logger:  logger,
		path:    dbPath,
		tempDir: tempDir,
	}

	// open database
	if err := db.open(); err != nil {
		return nil, err
	}

	// initialize with script if provided
	if opts.InitScript != "" {
		if err := db.Exec(opts.InitScript); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to initialize database: %w", err)
		}
	}

	logger.Debug("database created", "path", dbPath)

	return db, nil
}

// open opens the sqlite database
func (d *Database) open() error {
	db, err := sql.Open("sqlite3", d.path+"?_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	d.db = db
	return nil
}

// exec executes a sql statement
func (d *Database) Exec(query string, args ...interface{}) error {
	_, err := d.db.Exec(query, args...)
	return err
}

// query executes a query and returns rows
func (d *Database) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return d.db.Query(query, args...)
}

// queryrow executes a query and returns a single row
func (d *Database) QueryRow(query string, args ...interface{}) *sql.Row {
	return d.db.QueryRow(query, args...)
}

// getpath returns the database file path
func (d *Database) GetPath() string {
	return d.path
}

// getdb returns the underlying sql.db
func (d *Database) GetDB() *sql.DB {
	return d.db
}

// close closes the database and removes the file
func (d *Database) Close() error {
	if d.db != nil {
		d.db.Close()
	}

	// remove database files
	os.Remove(d.path)
	os.Remove(d.path + "-shm")
	os.Remove(d.path + "-wal")

	d.logger.Debug("database cleaned up", "path", d.path)

	return nil
}

// reset resets the database by removing all data
func (d *Database) Reset() error {
	// get all table names
	rows, err := d.db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		return fmt.Errorf("failed to get table names: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		tables = append(tables, name)
	}

	// delete all data from tables
	for _, table := range tables {
		if _, err := d.db.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			d.logger.Warn("failed to delete from table", "table", table, "error", err)
		}
	}

	d.logger.Debug("database reset")
	return nil
}

// tableexists checks if a table exists
func (d *Database) TableExists(tableName string) (bool, error) {
	var count int
	err := d.db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
		tableName,
	).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// getrowcount returns the number of rows in a table
func (d *Database) GetRowCount(tableName string) (int, error) {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	err := d.db.QueryRow(query).Scan(&count)
	return count, err
}

// importcsv imports a csv file into a table
func (d *Database) ImportCSV(tableName string, csvPath string) error {
	// TODO: implement csv import
	return fmt.Errorf("csv import not yet implemented")
}

// createtestdatabase creates a database with the standard schema
func CreateTestDatabase(ctx context.Context, logger *log.Logger) (*Database, error) {
	initScript := `
CREATE TABLE IF NOT EXISTS weather_data (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    station_id INTEGER NOT NULL,
    timestamp INTEGER NOT NULL,
    temperature REAL,
    humidity REAL,
    pressure REAL,
    wind_speed REAL,
    wind_direction REAL,
    precipitation REAL,
    location_lat REAL,
    location_lon REAL,
    data_quality INTEGER DEFAULT 0,
    source_file TEXT,
    imported_at INTEGER DEFAULT (strftime('%s', 'now')),
    UNIQUE(station_id, timestamp)
);

CREATE INDEX IF NOT EXISTS idx_weather_station_time ON weather_data(station_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_weather_time ON weather_data(timestamp);

CREATE TABLE IF NOT EXISTS ingest_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    completed_at INTEGER,
    records_processed INTEGER DEFAULT 0,
    records_failed INTEGER DEFAULT 0,
    status TEXT NOT NULL,
    error_message TEXT
);

CREATE TABLE IF NOT EXISTS hourly_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    station_id INTEGER NOT NULL,
    hour INTEGER NOT NULL,
    metric_name TEXT NOT NULL,
    min_val REAL,
    max_val REAL,
    avg_val REAL,
    count INTEGER,
    computed_at INTEGER DEFAULT (strftime('%s', 'now')),
    UNIQUE(station_id, hour, metric_name)
);

CREATE INDEX IF NOT EXISTS idx_hourly_station_time ON hourly_stats(station_id, hour);

CREATE TABLE IF NOT EXISTS peer_stations (
    station_id INTEGER PRIMARY KEY,
    hostname TEXT NOT NULL,
    ip_address TEXT,
    query_port INTEGER,
    replication_port INTEGER,
    first_seen INTEGER,
    last_seen INTEGER,
    last_beacon INTEGER,
    is_leader BOOLEAN DEFAULT 0,
    is_healthy BOOLEAN DEFAULT 0,
    capabilities INTEGER DEFAULT 0
);
`

	return NewDatabase(ctx, logger, DatabaseOptions{
		InitScript: initScript,
	})
}
