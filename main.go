package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	db *sql.DB
)

func main() {
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_DB"))

	db, err = sql.Open("postgres", connStr)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	db.Exec(`CREATE TABLE IF NOT EXISTS lessons (
    		id SERIAL PRIMARY KEY,
    		lessonNum TEXT NOT NULL,
    		lessonName TEXT NOT NULL,
    		substitute TEXT NOT NULL,
    		room TEXT NOT NULL,
    		additionalInfo TEXT NOT NULL,
    		teacher TEXT NOT NULL,
    		url TEXT NOT NULL
)`)

	fmt.Println("Starting...")

	for {
		checkSchedules()
		time.Sleep(5 * time.Minute)
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
				printLessonDetails(s, e.Request.URL.String())
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

	date := time.Now().AddDate(0, 0, daysToAdd).Format("2006-01-02")
	formattedURL := fmt.Sprintf(os.Getenv("SCRAPE_URL"), date)
	c.Visit(formattedURL)
}

func printLessonDetails(s *goquery.Selection, url string) {
	details := strings.Split(s.Text(), "\n")

	lessonNum := strings.TrimSpace(details[1])
	lessonName := strings.TrimSpace(details[2])
	substitute := strings.TrimSpace(details[3])
	room := strings.TrimSpace(details[4])
	additionalInfo := strings.TrimSpace(details[5])
	teacher := strings.TrimSpace(details[6])

	query := `SELECT EXISTS (SELECT 1 FROM lessons WHERE lessonNum=$1 AND lessonName=$2 AND substitute=$3 AND room=$4 AND additionalInfo=$5 AND teacher=$6 AND url=$7)`
	var exists bool
	err := db.QueryRow(query, lessonNum, lessonName, substitute, room, additionalInfo, teacher, url).Scan(&exists)
	if err != nil {
		panic(err)
	}

	if !exists {
		fmt.Println("[" + time.Now().Format(time.DateTime) + "] Details: " + strings.Join(details, " | ") + " | URL: " + url)

		insertQuery := `INSERT INTO lessons (lessonNum, lessonName, substitute, room, additionalInfo, teacher, url) VALUES ($1, $2, $3, $4, $5, $6, $7)`
		_, err := db.Exec(insertQuery, lessonNum, lessonName, substitute, room, additionalInfo, teacher, url)
		if err != nil {
			panic(err)
		}

		fmt.Println("[" + time.Now().Format(time.DateTime) + "] [nr " + lessonNum + "] Added to DB!")

		title := fmt.Sprintf("Lekcja: %s", lessonNum)
		description := fmt.Sprintf("Lekcja: `%s`\nZa: `%s`\nSala: `%s`\nDodatkowa informacja: `%s `\nZ: `%s`\nDzien: %s",
			lessonName, substitute, room, additionalInfo, teacher, url)

		err = sendEmbed(title, description, 0x00ff00)
		if err != nil {
			panic("Error sending embed: " + err.Error())
		}
		fmt.Println("Sent to Discord!")
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
