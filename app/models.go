package main

import (
        "database/sql"
        "time"

        "golang.org/x/crypto/bcrypt"
)

type User struct {
        ID       int
        Username string
        hash string
        Email    string
        Role     string
        Points   int
}

type Chore struct {
        ID     int
        Name   string
        Points int
}

type DailyChore struct {
        ID        int
        UserID    int
        ChoreID   int
        Date      time.Time
        Completed bool
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
        bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
        return string(bytes), err
}

// CheckPasswordHash compares a password with its hash
func CheckPasswordHash(password, hash string) bool {
        err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
        return err == nil
}

// GetUserByUsername retrieves a user by their username
func GetUserByUsername(db *sql.DB, username string) (*User, error) {
        row := db.QueryRow("SELECT id, username, hash, email, role, points FROM users WHERE username = ?", username)
        var user User
        err := row.Scan(&user.ID, &user.Username, &user.hash, &user.Email, &user.Role, &user.Points)
        if err != nil {
                return nil, err
        }
        return &user, nil
}

// CreateUser adds a new user to the database
func CreateUser(db *sql.DB, username, password, email, role string) error {
        hashedPassword, err := HashPassword(password)
        if err != nil {
                return err
        }

        _, err = db.Exec("INSERT INTO users (username, password, email, role) VALUES (?, ?, ?, ?)", username, hashedPassword, email, role)
        return err
}

// CreateChore adds a new chore to the database, including a default user
func CreateChore(db *sql.DB, name string, points int, defaultUserID int) error {
    _, err := db.Exec("INSERT INTO chores (name, points, default_user_id) VALUES (?, ?, ?)", name, points, defaultUserID)
    return err
}

// AssignChoreToUser assigns a chore to a user for a given date
func AssignChoreToUser(db *sql.DB, userID, choreID int, date string) error {
        _, err := db.Exec("INSERT INTO daily_chores (user_id, chore_id, date) VALUES (?, ?, ?)", userID, choreID, date)
        return err
}

