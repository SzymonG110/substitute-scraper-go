package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/joho/godotenv"
	"os"
	"strings"
	"time"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}

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
	fmt.Println("==============================================")
	fmt.Println()
	printScheduleForNextDay(c)
}

func printScheduleForToday(c *colly.Collector) {
	fmt.Println("Today:")
	formattedURL := fmt.Sprintf(os.Getenv("SCRAPE_URL"), time.Now().Format("2006-01-02"))
	c.Visit(formattedURL)
}

func printScheduleForNextDay(c *colly.Collector) {
	fmt.Println("Next day:")
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

	fmt.Println("Lekcja:", lessonNum)
	fmt.Println("Lekcja:", lessonName)
	fmt.Println("Za kogo:", substitute)
	fmt.Println("Sala:", room)
	fmt.Println("Dodatkowa informacja:", additionalInfo)
	fmt.Println("Z kim:", teacher)
	fmt.Println()
}
