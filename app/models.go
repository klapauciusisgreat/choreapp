package main

import (
        "database/sql"
	"fmt"
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

        _, err = db.Exec("INSERT INTO users (username, hash, email, role) VALUES (?, ?, ?, ?)", username, hashedPassword, email, role)
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

func GetDailyPoints(db *sql.DB, userID int, days int) (map[string]int, error) {
    dailyPoints := make(map[string]int)
    for i := 0; i < days; i++ {
        date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
        var points int
        err := db.QueryRow(`
            SELECT IFNULL(SUM(c.points), 0)
            FROM daily_chores dc
            JOIN chores c ON dc.chore_id = c.id
            WHERE dc.user_id = ? AND dc.date = ? AND dc.completed = TRUE
        `, userID, date).Scan(&points)
        if err != nil {
            return nil, fmt.Errorf("error getting daily points: %v", err)
        }
        dailyPoints[date] = points
    }
    return dailyPoints, nil
}

func GetWeeklyPoints(db *sql.DB, userID int, weeks int) (map[string]int, error) {
    weeklyPoints := make(map[string]int)
    for i := 0; i < weeks; i++ {
        startDate := time.Now().AddDate(0, 0, -i*7 - 6).Format("2006-01-02")
        endDate := time.Now().AddDate(0, 0, -i*7).Format("2006-01-02")
        var points int
        err := db.QueryRow(`
            SELECT IFNULL(SUM(c.points), 0)
            FROM daily_chores dc
            JOIN chores c ON dc.chore_id = c.id
            WHERE dc.user_id = ? AND dc.date BETWEEN ? AND ? AND dc.completed = TRUE
        `, userID, startDate, endDate).Scan(&points)
        if err != nil {
            return nil, fmt.Errorf("error getting weekly points: %v", err)
        }
        weeklyPoints[fmt.Sprintf("Week %d", i+1)] = points
    }
    return weeklyPoints, nil
}
