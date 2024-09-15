package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Define the global DB variable
var db *gorm.DB

// User struct with GORM tags and relationships
type User struct {
	ID        int     `gorm:"primaryKey" json:"id"`
	Name      string  `gorm:"type:varchar(100)" json:"name"`
	Gender    string  `gorm:"type:enum('male', 'female', 'other'); not null" json:"gender"`
	Latitude  float64 `gorm:"not null" json:"latitude"`
	Longitude float64 `gorm:"not null" json:"longitude"`
	DietType  string  `gorm:"type:varchar(50)" json:"diet_type"`
	Age       int     `gorm:"not null" json:"age"`

	// Relationships
	LikesSent       []Like  `gorm:"foreignKey:UserID" json:"likes_sent"`
	LikesReceived   []Like  `gorm:"foreignKey:LikedUserID" json:"likes_received"`
	MatchesSent     []Match `gorm:"foreignKey:UserID" json:"matches_sent"`
	MatchesReceived []Match `gorm:"foreignKey:MatchedUserID" json:"matches_received"`
}

// Like struct with GORM tags and relationships
type Like struct {
	ID          int       `gorm:"primaryKey" json:"id"`
	UserID      int       `gorm:"not null" json:"user_id"`
	LikedUserID int       `gorm:"not null" json:"liked_user_id"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	User      User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user"`
	LikedUser User `gorm:"foreignKey:LikedUserID;constraint:OnDelete:CASCADE" json:"liked_user"`
}

// Match struct with GORM tags and relationships
type Match struct {
	ID            int       `gorm:"primaryKey" json:"id"`
	UserID        int       `gorm:"not null" json:"user_id"`
	MatchedUserID int       `gorm:"not null" json:"matched_user_id"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	User        User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user"`
	MatchedUser User `gorm:"foreignKey:MatchedUserID;constraint:OnDelete:CASCADE" json:"matched_user"`
}

// Struct for the preferences
type Preferences struct {
	LookingForGender   string   `json:"looking_for_gender"`
	LookingForDietType string   `json:"looking_for_diet_type"`
	AgeRange           AgeRange `json:"age_range"`
	MaxDistance        float64  `json:"max_distance"`
}

type AgeRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// Register a like from one user to another and check for mutual like
func likeUser(c *gin.Context) {
	var like Like
	if err := c.ShouldBindJSON(&like); err == nil {
		like.CreatedAt = time.Now()
		db.Create(&like)

		// Check if there is a mutual like
		var mutualLike Like
		result := db.Where("user_id = ? AND liked_user_id = ?", like.LikedUserID, like.UserID).First(&mutualLike)

		if result.RowsAffected > 0 {
			// Create match object and save it to the DB
			createMatch(like.UserID, like.LikedUserID)
			c.JSON(http.StatusOK, gin.H{"status": "match found", "match": gin.H{"user_id": like.UserID, "matched_user_id": like.LikedUserID}})
		} else {
			// No mutual like yet
			c.JSON(http.StatusOK, gin.H{"status": "like registered"})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

// Create a match between two users
func createMatch(userID, likedUserID int) {
	match := Match{
		UserID:        userID,
		MatchedUserID: likedUserID,
		CreatedAt:     time.Now(),
	}
	db.Create(&match)
}

func getRecommendedUsers(c *gin.Context) {
	userID := c.Param("user_id")
	var requestingUser User
	if db.First(&requestingUser, userID).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var preferences Preferences
	if err := c.ShouldBindJSON(&preferences); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid preferences"})
		return
	}

	var recommendations []User
	queryBuilder := strings.Builder{}

	// Start the query
	queryBuilder.WriteString(`
        SELECT id, name, gender, latitude, longitude, diet_type, age,
            (3958.8 * acos(cos(radians(?)) * cos(radians(latitude)) * cos(radians(longitude) - radians(?)) + sin(radians(?)) * sin(radians(latitude)))) AS distance
        FROM users
        WHERE id != ?
    `)

	args := []interface{}{
		requestingUser.Latitude,
		requestingUser.Longitude,
		requestingUser.Latitude,
		requestingUser.ID,
	}

	// Validate preferences
	if preferences.LookingForGender != "" {
		queryBuilder.WriteString(" AND gender = ?")
		args = append(args, preferences.LookingForGender)
	}

	if preferences.LookingForDietType != "" {
		queryBuilder.WriteString(" AND diet_type = ?")
		args = append(args, preferences.LookingForDietType)
	}

	if preferences.AgeRange.Min > 0 && preferences.AgeRange.Max > 0 {
		if preferences.AgeRange.Min > preferences.AgeRange.Max {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid age range"})
			return
		}
		queryBuilder.WriteString(" AND age BETWEEN ? AND ?")
		args = append(args, preferences.AgeRange.Min, preferences.AgeRange.Max)
	}

	// Add distance filter
	queryBuilder.WriteString(" HAVING distance <= ?")
	if preferences.MaxDistance <= 0 {
		preferences.MaxDistance = 99999.0
	}
	args = append(args, preferences.MaxDistance)

	// Execute the dynamic query
	query := queryBuilder.String()
	if err := db.Debug().Raw(query, args...).Scan(&recommendations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving recommendations"})
		return
	}

	// Return the list of recommended users
	c.JSON(http.StatusOK, recommendations)
}

// API to retrieve matches for a user
func getMatches(c *gin.Context) {
	userID := c.Param("user_id")
	var matches []Match
	db.Where("user_id = ?", userID).Or("matched_user_id = ?", userID).Find(&matches)

	c.JSON(http.StatusOK, matches)
}

// Connect to the database using environment variables
func connectToDB() {
	log.Println("Connecting to the database...")
	// load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// Read database configuration from environment variables
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	if dbHost == "" {
		log.Fatal("DB_HOST environment variable is required")
	}
	if dbPort == "" {
		log.Fatal("DB_PORT environment variable is required")
	}
	if dbUser == "" {
		log.Fatal("DB_USER environment variable is required")
	}
	if dbPassword == "" {
		log.Fatal("DB_PASSWORD environment variable is required")
	}
	if dbName == "" {
		log.Fatal("DB_NAME environment variable is required")
	}

	// Build the DSN (Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect to database")
	}

	// Auto migrate the database schema
	db.AutoMigrate(&User{}, &Like{}, &Match{})
}

// Seed initial users into the database if they don't exist
func seedUsers() {
	var count int64
	db.Model(&User{}).Count(&count)

	if count == 0 {
		// Create default users
		users := []User{
			{Name: "Alice", Gender: "female", Latitude: 40.7128, Longitude: -74.0060, DietType: "vegan", Age: 25},
			{Name: "Bob", Gender: "male", Latitude: 40.73061, Longitude: -73.935242, DietType: "vegan", Age: 30},
			{Name: "Charlie", Gender: "other", Latitude: 41.033986, Longitude: -73.762909, DietType: "vegan", Age: 22},
			{Name: "Diana", Gender: "female", Latitude: 40.8501, Longitude: -73.8662, DietType: "vegan", Age: 28},
			{Name: "Ethan", Gender: "male", Latitude: 35.6895, Longitude: 139.6917, DietType: "omnivore", Age: 35},
		}

		// Insert the users into the database
		db.Create(&users)
		fmt.Println("Seeded initial users")
	} else {
		fmt.Println("Users already exist in the database, skipping seed")
	}
}

func main() {
	// Connect to the database
	connectToDB()

	// Seed users if they don't already exist
	seedUsers()

	// Initialize the Gin router
	r := gin.Default()

	// Route to like a user
	r.POST("/like", likeUser)

	// Route to get matches for a user
	r.GET("/matches/:user_id", getMatches)

	// Route to get recommended users for a user
	r.POST("/recommendations/:user_id", getRecommendedUsers)

	// Start the server
	r.Run(":8080")
}
