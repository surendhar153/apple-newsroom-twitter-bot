package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/joho/godotenv"
	"github.com/mmcdole/gofeed"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var db *gorm.DB
var twitterClient *twitter.Client
var feedUrl string

type Feed struct {
	gorm.Model
	URL         string
	PublishedAt *time.Time
	Title       string
	Content     string
	Category    string
}

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Twitter API credentials
	consumerKey := os.Getenv("CONSUMER_KEY")
	consumerSecret := os.Getenv("CONSUMER_SECRET")
	accessToken := os.Getenv("ACCESS_TOKEN")
	accessTokenSecret := os.Getenv("ACCESS_TOKEN_SECRET")

	// MySQL credentials
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")

	feedUrl = os.Getenv("FEED_URL")

	// Initialize the MySQL database connection

	db, err = gorm.Open(mysql.Open(dbUser+":"+dbPassword+"@tcp("+dbHost+":3306)/"+dbName+"?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	// defer db.Close() Not needed to close

	// Initialize the Twitter client
	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessTokenSecret)
	httpClient := config.Client(oauth1.NoContext, token)
	twitterClient = twitter.NewClient(httpClient)

	// Create the table to store the RSS feed history
	db.AutoMigrate(&Feed{})

	// Fetch and post the initial RSS feeds
	postInitialFeeds()

	// Check for new RSS updates every hour
	ticker := time.NewTicker(time.Hour)
	for range ticker.C {
		postNewFeeds()
	}
}

func postInitialFeeds() {
	// Fetch the RSS feed
	fp := gofeed.NewParser()
	feed, _ := fp.ParseURL(feedUrl)

	// Iterate through the feed items
	for i := len(feed.Items) - 1; i >= 0; i-- {
		item := feed.Items[i]

		// Check if the item has already been posted
		var count int64
		db.Model(&Feed{}).Where("url = ?", item.Link).Count(&count)

		if count == 0 {
			// Post the item to Twitter
			_, _, err := twitterClient.Statuses.Update(fmt.Sprintf("%s: %s\n#AppleNewsroom\n%s", item.Categories[0], item.Title, item.Link), nil)
			if err != nil {
				log.Fatal(err)
			}

			// Insert the item into the database
			var feed Feed
			feed.URL = item.Link
			feed.PublishedAt = item.UpdatedParsed // Apple uses only Updated value
			feed.Title = item.Title
			feed.Content = item.Content
			feed.Category = item.Categories[0]
			db.Create(&feed)
		}
	}
}

func postNewFeeds() {
	// Fetch the RSS feed
	fp := gofeed.NewParser()
	feed, _ := fp.ParseURL(feedUrl)

	// Find the latest published date in the database
	var latestPublishedAt time.Time
	err := db.Model(&Feed{}).Select("published_at").Order("published_at desc").Limit(1).Row().Scan(&latestPublishedAt)
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Fatal(err)
	}

	// Iterate through the feed items
	for _, item := range feed.Items {
		// Check if the item is newer than the latest item in the database
		if item.PublishedParsed.After(latestPublishedAt) {
			// Post the item to Twitter
			_, _, err := twitterClient.Statuses.Update(fmt.Sprintf("%s: %s\n#AppleNewsroom\n%s", item.Categories[0], item.Title, item.Link), nil)
			if err != nil {
				log.Fatal(err)
			}

			// Insert the item into the database
			var feed Feed
			feed.URL = item.Link
			feed.PublishedAt = item.UpdatedParsed // Apple uses only Updated value
			feed.Title = item.Title
			feed.Content = item.Content
			feed.Category = item.Categories[0]
			db.Create(&feed)
		}
	}
}
