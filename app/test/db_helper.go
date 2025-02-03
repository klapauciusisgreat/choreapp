package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func dropTables(db *sql.DB) error {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return err
		}
		if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS \"%s\"", tableName)); err != nil {
			return err
		}
	}
	return nil
}

func createTables(db *sql.DB) error {
	createTablesSQL := []string{
		`CREATE TABLE users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            username TEXT UNIQUE NOT NULL,
            hash TEXT NOT NULL,
            email TEXT NOT NULL,
            role TEXT NOT NULL,
            points INTEGER DEFAULT 0
          );`,
		`CREATE TABLE chores (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT UNIQUE NOT NULL,
            points INTEGER NOT NULL,
            default_user_id INTEGER,
            FOREIGN KEY (default_user_id) REFERENCES users(id)
          );`,
		`CREATE TABLE daily_chores (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            chore_id INTEGER NOT NULL,
            date DATE NOT NULL,
            completed BOOLEAN DEFAULT FALSE,
            UNIQUE(chore_id, date), -- Add this line
            FOREIGN KEY (user_id) REFERENCES users(id),
            FOREIGN KEY (chore_id) REFERENCES chores(id)
          );`,
	}

	for _, sql := range createTablesSQL {
		if _, err := db.Exec(sql); err != nil {
			return err
		}
	}
	return nil
}

func insertData(db *sql.DB) error {
	insertDataSQL := []string{
		`INSERT INTO users (username, hash, email, role, points) VALUES
            ('WitweBolte', '', 'bolte@wilhelmbusch.uk', 'parent', 15),
            ('Max', '', 'max@wilhelmbusch.uk', 'child', 15),
            ('Moritz', '', 'moritz@wilhelmbusch.uk', 'child', 6);`,
		`INSERT INTO chores (name, points, default_user_id) VALUES
            ('walk dog morning', 5, 1),
            ('walk dog afternoon', 5, 1),
            ('walk dog evening', 5, 1),
            ('Cat litter cleanup', 1, 3);`,
	}

	for _, sql := range insertDataSQL {
		if _, err := db.Exec(sql); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: db_helper {exec|reset} [db_path] <sqlite_command>")
		fmt.Println("db_path: Absolute path to the SQLite database file")
		os.Exit(1)
	}

	action := os.Args[1]
	dbPath := os.Args[2]

	switch action {
	case "exec":
		if len(os.Args) < 4 {
			fmt.Println("Error: Missing SQLite command.")
			fmt.Println("Usage: db_helper exec [db_path] \"<sqlite_command>\"")
			os.Exit(1)
		}
		sqliteCommand := os.Args[3]

		// Use a new connection for each exec command
		db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=rw", dbPath))
		if err != nil {
			fmt.Println("Error opening database:", err)
			os.Exit(1)
		}
		defer db.Close()

		rows, err := db.Query(sqliteCommand)
		if err != nil {
			fmt.Println("Error executing SQLite command:", err)
			os.Exit(1)
		}
		defer rows.Close()

		// Process and print results (this is a basic example)
		columns, err := rows.Columns()
		if err != nil {
			fmt.Println("Error getting columns:", err)
			os.Exit(1)
		}
		values := make([]sql.RawBytes, len(columns))
		scanArgs := make([]interface{}, len(values))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		for rows.Next() {
			err = rows.Scan(scanArgs...)
			if err != nil {
				fmt.Println("Error scanning row:", err)
				os.Exit(1)
			}

			var rowValues []string
			for _, col := range values {
				rowValues = append(rowValues, string(col))
			}
			fmt.Println(strings.Join(rowValues, ", "))
		}

	case "reset":
		// Open a new connection for creating tables and inserting data
		db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=rw", dbPath))
		if err != nil {
			fmt.Println("Error opening database:", err)
			os.Exit(1)
		}
		defer db.Close()

		fmt.Println("Dropping tables.")
		if err := dropTables(db); err != nil {
			fmt.Println("Error dropping tables:", err)
			os.Exit(1)
		}

		fmt.Println("Creating tables.")
		if err := createTables(db); err != nil {
			fmt.Println("Error creating tables:", err)
			os.Exit(1)
		}
		fmt.Println("inserting data.")

		if err := insertData(db); err != nil {
			fmt.Println("Error inserting data:", err)
			os.Exit(1)
		}

		fmt.Println("Database reset and populated.")

	default:
		fmt.Println("Usage: db_helper {exec|reset} [db_path] <sqlite_command>")
		os.Exit(1)
	}
}
