package main

import (
        "database/sql"
	"encoding/json"
        "fmt"
        "html/template"
        "log"
	"math/rand"
        "net/http"
        "net/smtp"
	"os"
        "time"
        "strconv"

        _ "github.com/mattn/go-sqlite3"
        //"golang.org/x/crypto/bcrypt"
)

// Database models - see models.go

var templates = template.Must(template.ParseGlob("app/templates/*.html"))
var db *sql.DB

// Simple session management (for demonstration purposes only)
var sessions = make(map[string]int) // Session ID -> User ID

// generateSessionID generates a simple random session ID
func generateSessionID() string {
        b := make([]byte, 16)
        _, err := rand.Read(b)
        if err != nil {
                // Handle error appropriately in a real application
                panic(err)
        }
        return fmt.Sprintf("%x", b)
}

func main() {
        // Database setup
        var err error
        db, err = sql.Open("sqlite3", "./db/chores.db")
        if err != nil {
                log.Fatal(err)
        }
        defer db.Close()

        // Create tables if they don't exist
	_, err = db.Exec(`
          CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            username TEXT UNIQUE NOT NULL,
            hash TEXT NOT NULL,
            email TEXT NOT NULL,
            role TEXT NOT NULL,
            points INTEGER DEFAULT 0
          );

          CREATE TABLE IF NOT EXISTS chores (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT UNIQUE NOT NULL,
            points INTEGER NOT NULL,
            default_user_id INTEGER,
            FOREIGN KEY (default_user_id) REFERENCES users(id)
          );

          CREATE TABLE IF NOT EXISTS daily_chores (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            chore_id INTEGER NOT NULL,
            date DATE NOT NULL,
            completed BOOLEAN DEFAULT FALSE,
            FOREIGN KEY (user_id) REFERENCES users(id),
            FOREIGN KEY (chore_id) REFERENCES chores(id)
          );
        `)
        if err != nil {
		log.Fatalf("Failed to create tables: %v", err)
        }

	// Serve static files (CSS, JS, images, etc.)
	fs := http.FileServer(http.Dir("./app/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
        // HTTP Handlers
        http.HandleFunc("/", indexHandler)
        http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
        http.HandleFunc("/user/create", createUserHandler)
        http.HandleFunc("/chore/create", createChoreHandler)
        http.HandleFunc("/chore/assign", assignChoreHandler)
	http.HandleFunc("/chore/claim", claimChoreHandler)
        http.HandleFunc("/chore/update", choreUpdateHandler)
	http.HandleFunc("/chores", getChoresHandler)
	http.HandleFunc("/points", getPointsHandler)




        // Scheduled tasks (daily and weekly summaries)
        go scheduleDailySummary(db)
        go scheduleWeeklySummary(db)

	// Start the HTTPS server
	log.Println("Server starting on port 443")
	log.Fatal(http.ListenAndServeTLS(":443",
		"/app/certbot/config/live/"+os.Getenv("DUCKDNS_SUBDOMAIN")+
			".duckdns.org/fullchain.pem",
		"/app/certbot/config/live/"+os.Getenv("DUCKDNS_SUBDOMAIN")+
			".duckdns.org/privkey.pem",
		nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
    user := getCurrentUser(r)
    if user == nil {
        http.Redirect(w, r, "/login", http.StatusFound)
        return
    }

    // Assign chores to default owners if not already assigned
    today := time.Now().Format("2006-01-02")
    _, err := db.Exec(`
        INSERT INTO daily_chores (user_id, chore_id, date)
        SELECT c.default_user_id, c.id, ?
        FROM chores c
        LEFT JOIN daily_chores dc ON c.id = dc.chore_id AND dc.date = ?
        WHERE dc.id IS NULL
    `, today, today)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Get daily points for the preceding week
    dailyPoints, err := GetDailyPoints(db, user.ID, 7)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Get weekly points for the preceding 4 weeks
    weeklyPoints, err := GetWeeklyPoints(db, user.ID, 4)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
	
    data := struct {
        User          *User
        DailyPoints   map[string]int
        WeeklyPoints  map[string]int
        CurrentUserID int
    }{
        User:          user,
        DailyPoints:   dailyPoints,
        WeeklyPoints:  weeklyPoints,
        CurrentUserID: user.ID,
    }

    templates.ExecuteTemplate(w, "index.html", data)
}

// getCurrentUser retrieves the current user from the session
func getCurrentUser(r *http.Request) *User {
        cookie, err := r.Cookie("session_id")
        if err != nil {
                return nil // No session cookie found
        }

        sessionID := cookie.Value
        userID, ok := sessions[sessionID]
        if !ok {
                return nil // Session ID not found
        }

        // Fetch the user from the database
        user, err := GetUserByID(db, userID) // You'll need to implement GetUserByID
        if err != nil {
                return nil // Error fetching user
        }


        return user
}

// GetUserByID retrieves a user by their ID
func GetUserByID(db *sql.DB, id int) (*User, error) {
        row := db.QueryRow("SELECT id, username, hash, email, role, points FROM users WHERE id = ?", id)
        var user User
        err := row.Scan(&user.ID, &user.Username, &user.hash, &user.Email, &user.Role, &user.Points)
        if err != nil {
                return nil, err
        }
        return &user, nil
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
        if r.Method == "POST" {
                username := r.FormValue("username")
                password := r.FormValue("password")

                user, err := GetUserByUsername(db, username)
                if err != nil {
                        http.Error(w, "Invalid credentials", http.StatusUnauthorized)
                        return
                }
                if !CheckPasswordHash(password, user.hash) {
                        http.Error(w, "Invalid credentials", http.StatusUnauthorized)
                        return
                }

                // Create a new session
                sessionID := generateSessionID()
                sessions[sessionID] = user.ID

                // Set the session ID in a cookie
                http.SetCookie(w, &http.Cookie{
                        Name:     "session_id",
                        Value:    sessionID,
                        HttpOnly: true,
                        Secure:   true, // Should be true in production (requires HTTPS)
                        SameSite: http.SameSiteStrictMode,
                        Path:     "/",
                })

                http.Redirect(w, r, "/", http.StatusFound)
        } else {
                templates.ExecuteTemplate(w, "login.html", nil)
        }
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
        cookie, err := r.Cookie("session_id")
        if err != nil {
                // No session cookie, so no need to logout
                http.Redirect(w, r, "/", http.StatusFound)
                return
        }

        sessionID := cookie.Value

        // Remove the session from the sessions map
        delete(sessions, sessionID)

        // Expire the session cookie in the browser
        http.SetCookie(w, &http.Cookie{
                Name:     "session_id",
                Value:    "",
                Expires:  time.Unix(0, 0),
                HttpOnly: true,
                Secure:   true, // Should be true in production (requires HTTPS)
                SameSite: http.SameSiteStrictMode,
                Path:     "/",
        })

        http.Redirect(w, r, "/login", http.StatusFound)
}



func createUserHandler(w http.ResponseWriter, r *http.Request) {
        if r.Method == "POST" {
                username := r.FormValue("username")
                password := r.FormValue("password")
                email := r.FormValue("email")
                role := r.FormValue("role")

                err := CreateUser(db, username, password, email, role)
                if err != nil {
                        http.Error(w, err.Error(), http.StatusInternalServerError)
                        return
                }

                // Redirect to a success page or back to the user list
                http.Redirect(w, r, "/", http.StatusFound)
        } else {
                // Render a form to create a user (you'll need to create a corresponding HTML template)
                templates.ExecuteTemplate(w, "create_user.html", nil)
        }
}

func createChoreHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		name := r.FormValue("name")
		points, err := strconv.Atoi(r.FormValue("points"))
		if err != nil {
			http.Error(w, "Invalid points value", http.StatusBadRequest)
			return
		}
		defaultUserID, err := strconv.Atoi(r.FormValue("default_user_id"))
		if err != nil {
			http.Error(w, "Invalid default user ID", http.StatusBadRequest)
			return
		}

		err = CreateChore(db, name, points, defaultUserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Redirect to a success page or back to the chore list
		http.Redirect(w, r, "/", http.StatusFound)
	} else {
		userRows, err := db.Query("SELECT id, username FROM users")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer userRows.Close()

		var users []User
		for userRows.Next() {
			var user User
			if err := userRows.Scan(&user.ID, &user.Username); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			users = append(users, user)
		}

		// Render a form to create a chore, passing users for the dropdown
		templates.ExecuteTemplate(w, "create_chore.html", struct{ Users []User }{Users: users})
	}
}
        // Render a form to create a chore (you'll need a corresponding HTML tem

func assignChoreHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == "POST" {
        userID, err := strconv.Atoi(r.FormValue("user_id"))
        if err != nil {
            http.Error(w, "Invalid user ID", http.StatusBadRequest)
            return
        }
        choreID, err := strconv.Atoi(r.FormValue("chore_id"))
        if err != nil {
            http.Error(w, "Invalid chore ID", http.StatusBadRequest)
            return
        }

        // Parse and format the date correctly
        date, err := time.Parse("2006-01-02", r.FormValue("date"))
        if err != nil {
            http.Error(w, "Invalid date format", http.StatusBadRequest)
            return
        }
        formattedDate := date.Format("2006-01-02") // Format date for database

        err = AssignChoreToUser(db, userID, choreID, formattedDate) // Use formatted date
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        http.Redirect(w, r, "/", http.StatusFound)
    } else {
        // Get all users
        userRows, err := db.Query("SELECT id, username FROM users")
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        defer userRows.Close()

        var users []User
        for userRows.Next() {
            var user User
            if err := userRows.Scan(&user.ID, &user.Username); err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            users = append(users, user)
        }

        // Get all chores
        choreRows, err := db.Query("SELECT id, name FROM chores")
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        defer choreRows.Close()

        var chores []Chore
        for choreRows.Next() {
            var chore Chore
            if err := choreRows.Scan(&chore.ID, &chore.Name); err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            chores = append(chores, chore)
        }

        // Pass users and chores to the template
        data := struct {
            Users  []User
            Chores []Chore
        }{
            Users:  users,
            Chores: chores,
        }

        templates.ExecuteTemplate(w, "assign_chore.html", data)
    }
}
func getChoresHandler(w http.ResponseWriter, r *http.Request) {
    user := getCurrentUser(r)
    if user == nil {
        http.Error(w, "User not logged in", http.StatusUnauthorized)
        return
    }

    today := time.Now().Format("2006-01-02")
    allChores, err := fetchChoresData(db, user.ID, today)
    if err != nil {
        log.Printf("Error fetching chores data: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(allChores); err != nil {
        log.Printf("Error encoding chores to JSON: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}

func fetchChoresData(db *sql.DB, userID int, today string) ([]struct {
    ID          int
    Completed   bool
    Name        string
    Points      int
    UserID      sql.NullInt64
    IsAssigned  bool
    IsClaimable bool
}, error) {
    rows, err := db.Query(`
        SELECT
            c.id,
            dc.completed,
            c.name,
            c.points,
            dc.user_id,
            CASE WHEN dc.user_id = ? THEN 1 ELSE 0 END AS is_assigned,
            CASE WHEN (dc.user_id <> ? OR dc.user_id IS NULL) AND (dc.completed = FALSE OR dc.completed IS NULL) THEN 1 ELSE 0 END AS is_claimable
        FROM chores c
        LEFT JOIN daily_chores dc ON c.id = dc.chore_id AND dc.date = ?
        WHERE (dc.user_id = ? OR dc.user_id IS NULL OR dc.user_id <> ?)
    `, userID, userID, today, userID, userID)
    if err != nil {
        return nil, fmt.Errorf("error getting chores: %v", err)
    }
    defer rows.Close()

    var allChores []struct {
        ID          int
        Completed   bool
        Name        string
        Points      int
        UserID      sql.NullInt64
        IsAssigned  bool
        IsClaimable bool
    }
    for rows.Next() {
        var chore struct {
            ID          int
            Completed   bool
            Name        string
            Points      int
            UserID      sql.NullInt64
            IsAssigned  bool
            IsClaimable bool
        }
        if err := rows.Scan(&chore.ID, &chore.Completed, &chore.Name, &chore.Points, &chore.UserID, &chore.IsAssigned, &chore.IsClaimable); err != nil {
            return nil, fmt.Errorf("error scanning chore: %v", err)
        }
        allChores = append(allChores, chore)
    }

    return allChores, nil
}



func choreUpdateHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    user := getCurrentUser(r)
    if user == nil {
        http.Redirect(w, r, "/login", http.StatusFound)
        return
    }

    choreID, err := strconv.Atoi(r.FormValue("chore_id"))
    if err != nil {
        http.Error(w, "Invalid chore ID", http.StatusBadRequest)
        return
    }

    // Get the desired completion status from the form (default to false)
    completedStr := r.FormValue("completed")
    completed := completedStr == "true" // Convert string to boolean

    today := time.Now().Format("2006-01-02")

    // Update the chore's completion status in the database
    _, err = db.Exec(`
        UPDATE daily_chores
        SET completed = ?
        WHERE user_id = ? AND chore_id = ? AND date = ?
    `, completed, user.ID, choreID, today)
    if err != nil {
        log.Printf("Error updating chore completion status: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Adjust points based on completion status
    var points int
    err = db.QueryRow(`
        SELECT points FROM chores WHERE id = ?
    `, choreID).Scan(&points)
    if err != nil {
        log.Printf("Error getting chore points: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    if completed {
        // Add points if chore is marked as completed
        _, err = db.Exec(`
            UPDATE users SET points = points + ?
            WHERE id = ?
        `, points, user.ID)
    } else {
        // Subtract points if chore is marked as incomplete
        _, err = db.Exec(`
            UPDATE users SET points = points - ?
            WHERE id = ?
        `, points, user.ID)
    }
    if err != nil {
        log.Printf("Error updating user points: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Fetch updated chores data
    updatedChores, err := fetchChoresData(db, user.ID, today)
    if err != nil {
        log.Printf("Error fetching updated chores data: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Respond with JSON
    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(updatedChores); err != nil {
        log.Printf("Error encoding updated chores to JSON: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}

func claimChoreHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    user := getCurrentUser(r)
    if user == nil {
        log.Printf("User not logged in, redirecting to /login")
        http.Redirect(w, r, "/login", http.StatusFound)
        return
    }

    choreID, err := strconv.Atoi(r.FormValue("chore_id"))
    if err != nil {
        http.Error(w, "Invalid chore ID", http.StatusBadRequest)
        return
    }

    today := time.Now().Format("2006-01-02")

	
    // Use REPLACE INTO to insert or replace the chore assignment
    _, err = db.Exec(`
        REPLACE INTO daily_chores (user_id, chore_id, date)
        VALUES (?, ?, ?)
    `, user.ID, choreID, today)
    if err != nil {
        log.Printf("Error inserting or replacing chore: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Fetch updated chores data
    updatedChores, err := fetchChoresData(db, user.ID, today)
    if err != nil {
        log.Printf("Error fetching updated chores data: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Respond with JSON
	
    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(updatedChores); err != nil {
	log.Printf("Error encoding updated chores to JSON: %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}
func getPointsHandler(w http.ResponseWriter, r *http.Request) {
    user := getCurrentUser(r)
    if user == nil {
        http.Error(w, "User not logged in", http.StatusUnauthorized)
        return
    }

	//    today := time.Now().Format("2006-01-02")

    // Get daily points for the preceding week
    dailyPoints := make(map[string]int)
    dailyData := make([]int, 7)
    for i := 0; i < 7; i++ {
        date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
        var points int
        err := db.QueryRow(`
            SELECT IFNULL(SUM(c.points), 0)
            FROM daily_chores dc
            JOIN chores c ON dc.chore_id = c.id
            WHERE dc.user_id = ? AND dc.date = ? AND dc.completed = TRUE
        `, user.ID, date).Scan(&points)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        dailyPoints[date] = points
        dailyData[6-i] = points // Fill the array in reverse order (older to newer)
    }

    // Get weekly points for the preceding 4 weeks
    weeklyPoints := make(map[string]int)
    weeklyData := make([]int, 4)
    for i := 0; i < 4; i++ {
        startDate := time.Now().AddDate(0, 0, -i*7 - 6).Format("2006-01-02")
        endDate := time.Now().AddDate(0, 0, -i*7).Format("2006-01-02")
        var points int
        err := db.QueryRow(`
            SELECT IFNULL(SUM(c.points), 0)
            FROM daily_chores dc
            JOIN chores c ON dc.chore_id = c.id
            WHERE dc.user_id = ? AND dc.date BETWEEN ? AND ? AND dc.completed = TRUE
        `, user.ID, startDate, endDate).Scan(&points)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        weeklyPoints[fmt.Sprintf("Week %d", i+1)] = points
        weeklyData[3-i] = points // Fill the array in reverse order (older to newer)
    }

    pointsData := struct {
        DailyData  []int `json:"dailyData"`
        WeeklyData []int `json:"weeklyData"`
    }{
        DailyData:  dailyData,
        WeeklyData: weeklyData,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(pointsData)
}


// Email functions

func sendChoreCompletionEmail(user *User, choreName string) {
        // Construct email message
        from := "your_email@example.com" // Replace with your email
        password := "your_app_password"    // Replace with your app password
        to := []string{"parent_email@example.com"} // Replace with parent's email
        subject := "Chore Completed: " + choreName
        body := fmt.Sprintf("Hello,\n\n%s has completed the chore: %s\n\n", user.Username, choreName)

        msg := []byte("To: " + to[0] + "\r\n" +
                "Subject: " + subject + "\r\n" +
                "\r\n" +
                body + "\r\n")

        // SMTP authentication
        auth := smtp.PlainAuth("", from, password, "smtp.gmail.com")

        // Send email
        err := smtp.SendMail("smtp.gmail.com:587", auth, from, to, msg)
        if err != nil {
                log.Printf("Error sending email: %v", err)
        }
}

func scheduleDailySummary(db *sql.DB) {
        // Set the time for the first execution (e.g., 11:59 PM)
        firstExecution := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 23, 59, 0, 0, time.Local)
        if time.Now().After(firstExecution) {
                // If it's already past 11:59 PM today, schedule for tomorrow
                firstExecution = firstExecution.Add(24 * time.Hour)
        }

        durationUntilFirstExecution := firstExecution.Sub(time.Now())

        // Wait until the first execution time
        time.Sleep(durationUntilFirstExecution)

        // Execute the first task
        sendDailySummaryEmails(db)

        // Schedule the task to run every 24 hours
        ticker := time.NewTicker(24 * time.Hour)
        defer ticker.Stop()

        for range ticker.C {
                sendDailySummaryEmails(db)
        }
}

func sendDailySummaryEmails(db *sql.DB) {
        // Get all users
        rows, err := db.Query("SELECT id, username, email, points FROM users")
        if err != nil {
                log.Printf("Error fetching users: %v", err)
                return
        }
        defer rows.Close()

        var users []User
        for rows.Next() {
                var user User
                if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Points); err != nil {
                        log.Printf("Error scanning user: %v", err)
                        continue
                }
                users = append(users, user)
        }

        // Get today's completed chores
        today := time.Now().Format("2006-01-02")
        choreRows, err := db.Query(`
        SELECT dc.user_id, u.username, c.name, c.points
        FROM daily_chores dc
        JOIN users u ON dc.user_id = u.id
        JOIN chores c ON dc.chore_id = c.id
        WHERE dc.date = ? AND dc.completed = TRUE`, today)
        if err != nil {
                log.Printf("Error fetching daily chores: %v", err)
                return
        }
        defer choreRows.Close()

        // Create a map to store each user's completed chores
        userChores := make(map[int][]struct {
                Username   string
                ChoreName  string
                ChorePoints int
        })

        for choreRows.Next() {
                var userID int
                var username, choreName string
                var chorePoints int
                if err := choreRows.Scan(&userID, &username, &choreName, &chorePoints); err != nil {
                        log.Printf("Error scanning daily chore: %v", err)
                        continue
                }
                userChores[userID] = append(userChores[userID], struct {
                        Username   string
                        ChoreName  string
                        ChorePoints int
                }{username, choreName, chorePoints})
        }

        // Send email to each user
        for _, user := range users {
                var body string
                if user.Role == "child" {
                        body = fmt.Sprintf("Hello %s,\n\n", user.Username)
                        if chores, ok := userChores[user.ID]; ok {
                                body += "Here are the chores you completed today:\n"
                                for _, chore := range chores {
                                        body += fmt.Sprintf("- %s (%d points)\n", chore.ChoreName, chore.ChorePoints)
                                }
                                body += fmt.Sprintf("\nTotal points earned today: %d\n", user.Points)
                        } else {
                                body += "You did not complete any chores today.\n"
                        }
                } else if user.Role == "parent" {
                        body = "Hello,\n\nHere is the summary of completed chores today:\n"
                        for _, user := range users {
                                if user.Role == "child" {
                                        body += fmt.Sprintf("\n%s:\n", user.Username)
                                        if chores, ok := userChores[user.ID]; ok {
                                                for _, chore := range chores {
                                                        body += fmt.Sprintf("- %s (%d points)\n", chore.ChoreName, chore.ChorePoints)
                                                }
                                                body += fmt.Sprintf("Total points earned today: %d\n", user.Points)
                                        } else {
                                                body += "No chores completed today.\n"
                                        }
                                }
                        }
                }

                // Send email
                if body != "" {
                        from := "your_email@example.com"
                        password := "your_password"
                        to := []string{user.Email}
                        subject := "Daily Chore Summary"

                        msg := []byte("To: " + to[0] + "\r\n" +
                                "Subject: " + subject + "\r\n" +
                                "\r\n" +
                                body + "\r\n")

                        auth := smtp.PlainAuth("", from, password, "smtp.gmail.com")

                        err := smtp.SendMail("smtp.gmail.com:587", auth, from, to, msg)
                        if err != nil {
                                log.Printf("Error sending email to %s: %v", user.Email, err)
                        }
                }
        }
}

func scheduleWeeklySummary(db *sql.DB) {
        // Calculate the time for the first execution (e.g., Sunday at 11:59 PM)
        now := time.Now()
        daysUntilSunday := int(time.Sunday - now.Weekday())
        if daysUntilSunday < 0 {
                daysUntilSunday += 7 // If today is Sunday, the difference will be 0
        }
        firstExecution := time.Date(now.Year(), now.Month(), now.Day()+daysUntilSunday, 23, 59, 0, 0, time.Local)
        if now.After(firstExecution) {
                // If it's already past the first execution time this week, schedule for next week
                firstExecution = firstExecution.AddDate(0, 0, 7)
        }

        durationUntilFirstExecution := firstExecution.Sub(now)

        // Wait until the first execution time
        time.Sleep(durationUntilFirstExecution)

        // Execute the first task
        sendWeeklySummaryEmails(db)

        // Schedule the task to run every week
        ticker := time.NewTicker(7 * 24 * time.Hour)
        defer ticker.Stop()

        for range ticker.C {
                sendWeeklySummaryEmails(db)
        }
}

func sendWeeklySummaryEmails(db *sql.DB) {
        // Get all users
        rows, err := db.Query("SELECT id, username, email, role FROM users")
        if err != nil {
                log.Printf("Error fetching users: %v", err)
                return
        }
        defer rows.Close()

        var users []User
        for rows.Next() {
                var user User
                if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Role); err != nil {
                        log.Printf("Error scanning user: %v", err)
                        continue
                }
                users = append(users, user)
        }

        // Calculate start and end of the last week
        now := time.Now()
        endOfWeek := now.AddDate(0, 0, -int(now.Weekday()))            // Go back to the last Sunday
        startOfWeek := endOfWeek.AddDate(0, 0, -6)                     // Go back 6 more days for the start of the week
        formattedStartOfWeek := startOfWeek.Format("2006-01-02")
        formattedEndOfWeek := endOfWeek.Format("2006-01-02")

        // Get the last week's completed chores
        choreRows, err := db.Query(`
        SELECT dc.user_id, u.username, c.name, c.points, dc.date
        FROM daily_chores dc
        JOIN users u ON dc.user_id = u.id
        JOIN chores c ON dc.chore_id = c.id
        WHERE dc.date BETWEEN ? AND ? AND dc.completed = TRUE`, formattedStartOfWeek, formattedEndOfWeek)
        if err != nil {
                log.Printf("Error fetching weekly chores: %v", err)
                return
        }
        defer choreRows.Close()

        // Create a map to store each user's weekly completed chores
        userWeeklyChores := make(map[int]map[string][]struct {
                ChoreName  string
                ChorePoints int
        })

        for choreRows.Next() {
                var userID int
                var username, choreName, choreDate string
                var chorePoints int
                if err := choreRows.Scan(&userID, &username, &choreName, &chorePoints, &choreDate); err != nil {
                        log.Printf("Error scanning weekly chore: %v", err)
                        continue
                }

                // Initialize the map for the user and date if it doesn't exist
                if _, ok := userWeeklyChores[userID]; !ok {
                        userWeeklyChores[userID] = make(map[string][]struct {
                                ChoreName  string
                                ChorePoints int
                        })
                }
                if _, ok := userWeeklyChores[userID][choreDate]; !ok {
                        userWeeklyChores[userID][choreDate] = []struct {
                                ChoreName  string
                                ChorePoints int
                        }{}
                }

                userWeeklyChores[userID][choreDate] = append(userWeeklyChores[userID][choreDate], struct {
                        ChoreName  string
                        ChorePoints int
                }{choreName, chorePoints})
        }

        // Send email to each user
        for _, user := range users {
                var body string
                if user.Role == "child" {
                        body = fmt.Sprintf("Hello %s,\n\n", user.Username)
                        weeklyPoints := 0
                        if weeklyChores, ok := userWeeklyChores[user.ID]; ok {
                                body += "Here are the chores you completed last week:\n\n"
                                for date, chores := range weeklyChores {
                                        body += fmt.Sprintf("%s:\n", date)
                                        for _, chore := range chores {
                                                body += fmt.Sprintf("- %s (%d points)\n", chore.ChoreName, chore.ChorePoints)
                                                weeklyPoints += chore.ChorePoints
                                        }
                                        body += "\n"
                                }
                                body += fmt.Sprintf("Total points earned last week: %d\n", weeklyPoints)
                                body += fmt.Sprintf("Total allowance earned last week: $%.2f\n", float64(weeklyPoints)*0.1) // Assuming 1 point = $0.1
                        } else {
                                body += "You did not complete any chores last week.\n"
                        }
                } else if user.Role == "parent" {
                        body = "Hello,\n\nHere is the summary of completed chores last week:\n\n"
                        for _, child := range users {
                                if child.Role == "child" {
                                        body += fmt.Sprintf("%s:\n", child.Username)
                                        weeklyPoints := 0
                                        if weeklyChores, ok := userWeeklyChores[child.ID]; ok {
                                                for date, chores := range weeklyChores {
                                                        body += fmt.Sprintf("%s:\n", date)
                                                        for _, chore := range chores {
                                                                body += fmt.Sprintf("- %s (%d points)\n", chore.ChoreName, chore.ChorePoints)
                                                                weeklyPoints += chore.ChorePoints
                                                        }
                                                        body += "\n"
                                                }
                                                body += fmt.Sprintf("Total points earned last week: %d\n", weeklyPoints)
                                                body += fmt.Sprintf("Total allowance earned last week: $%.2f\n\n", float64(weeklyPoints)*0.1) // Assuming 1 point = $0.1
                                        } else {
                                                body += "No chores completed last week.\n\n"
                                        }
                                }
                        }
                }

                // Send email
                if body != "" {
                        from := "your_email@example.com"
                        password := "your_password"
                        to := []string{user.Email}
                        subject := "Weekly Chore Summary"

                        msg := []byte("To: " + to[0] + "\r\n" +
                                "Subject: " + subject + "\r\n" +
                                "\r\n" +
                                body + "\r\n")

                        auth := smtp.PlainAuth("", from, password, "smtp.gmail.com")

                        err := smtp.SendMail("smtp.gmail.com:587", auth, from, to, msg)
                        if err != nil {
                                log.Printf("Error sending email to %s: %v", user.Email, err)
                        }
                }
        }

        // Reset points for all users at the end of the week
        _, err = db.Exec("UPDATE users SET points = 0")
        if err != nil {
                log.Printf("Error resetting user points: %v", err)
        }
}
