package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	database *mongo.Database
	ep       *mongo.Collection
)

func main() {
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}

	client, err := mongo.Connect(context.TODO(), options.Client().
		ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	database = client.Database(os.Getenv("MONGO_DB"))
	ep = database.Collection(os.Getenv("MONGO_COLLECTION"))

	for {
		checkSchedules()
		time.Sleep(3 * time.Second)
	}
}

func checkSchedules() {
	c := colly.NewCollector()

	c.OnHTML("tr.table-active", func(e *colly.HTMLElement) {
		if !strings.Contains(e.Text, os.Getenv("CLASS_NAME")) {
			return
		}

		e.DOM.NextAll().EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if strings.HasPrefix(s.Text(), "Klasa") {
				return false
			}

			if strings.TrimSpace(s.Text()) != "" {
				printLessonDetails(s)
			}
			return true
		})
	})

	printScheduleForToday(c)
	printScheduleForNextDay(c)
}

func printScheduleForToday(c *colly.Collector) {
	formattedURL := fmt.Sprintf(os.Getenv("SCRAPE_URL"), time.Now().Format("2006-01-02"))
	c.Visit(formattedURL)
}

func printScheduleForNextDay(c *colly.Collector) {
	daysToAdd := 1
	if time.Now().Weekday() == time.Friday {
		daysToAdd = 3
	}
	if time.Now().Weekday() == time.Saturday {
		daysToAdd = 2
	}
	formattedURL := fmt.Sprintf(os.Getenv("SCRAPE_URL"), time.Now().AddDate(0, 0, daysToAdd).Format("2006-01-02"))
	c.Visit(formattedURL)
}

func printLessonDetails(s *goquery.Selection) {
	details := strings.Split(s.Text(), "\n")

	lessonNum := strings.TrimSpace(details[1])
	lessonName := strings.TrimSpace(details[2])
	substitute := strings.TrimSpace(details[3])
	room := strings.TrimSpace(details[4])
	additionalInfo := strings.TrimSpace(details[5])
	teacher := strings.TrimSpace(details[6])

	res := ep.FindOne(context.TODO(), bson.D{
		{"lessonNum", lessonNum},
		{"lessonName", lessonName},
		{"substitute", substitute},
		{"room", room},
		{"additionalInfo", additionalInfo},
		{"teacher", teacher},
	})
	if errors.Is(res.Err(), mongo.ErrNoDocuments) {
		_, err := ep.InsertOne(context.TODO(), bson.D{
			{"lessonNum", lessonNum},
			{"lessonName", lessonName},
			{"substitute", substitute},
			{"room", room},
			{"additionalInfo", additionalInfo},
			{"teacher", teacher},
		})
		if err != nil {
			panic(err)
		}

		title := fmt.Sprintf("Lekcja: %s", lessonNum)
		description := fmt.Sprintf("Lekcja: `%s`\nZa: `%s`\nSala: `%s`\nDodatkowa informacja: `%s`\nZ: `%s`",
			lessonName, substitute, room, additionalInfo, teacher)

		err = sendEmbed(title, description, 0x00ff00)
		if err != nil {
			panic("Error sending embed: " + err.Error())
		}
	}
}

type Embed struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Color       int    `json:"color,omitempty"`
}

type WebhookMessage struct {
	Content string  `json:"content,omitempty"`
	Embeds  []Embed `json:"embeds,omitempty"`
}

func sendEmbed(title, description string, color int) error {
	embed := Embed{
		Title:       title,
		Description: description,
		Color:       color,
	}

	message := WebhookMessage{
		Content: os.Getenv("WEBHOOK_MESSAGE_CONTENT"),
		Embeds:  []Embed{embed},
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	resp, err := http.Post(os.Getenv("WEBHOOK_URL"), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("non-OK HTTP status: %s", resp.Status)
	}

	return nil
}
