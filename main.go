package main

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-redis/redis/v7"
)

type Post struct {
	Number      int      `json:"number"`
	Title       string   `json:"title"`
	Description string   `json:"content"`
	Thumbnail   string   `json:"thumbnail"`
	Images      []string `json:"images"`
	Updated     string   `json"updated"`
}

type Pack struct {
	Messages []Post
}

var hash = map[string]int{}

func RequestList(url string) {
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	if res.StatusCode != 200 {
		log.Fatal(res.Status)
	}
	doc, err := goquery.NewDocumentFromResponse(res)
	if err != nil {
		log.Fatal(err)
	}

	current := map[string]int{}

	doc.Find(".gall_list > tbody").Children().Each(func(i int, s *goquery.Selection) {
		if dataType, exist := s.Attr("data-type"); exist && dataType != "icon_notice" {
			href, _ := s.Find(".gall_tit > a").Attr("href")
			number, _ := strconv.Atoi(s.Find(".gall_num").Text())
			current[href] = number
		}
	})

	var pack Pack = Pack{}
	for key, number := range current {
		if _, exist := hash[key]; !exist {
			post := RequestPost("http://gall.dcinside.com" + key)
			post.Number = number

			pack.Messages = append(pack.Messages, post)
		}
	}

	hash = current

	go Publish(pack)
}

func RequestPost(url string) Post {
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Googlebot")

	httpClient := &http.Client{}
	res, err := httpClient.Do(req)

	if err != nil {
		log.Println(err)
	}
	if res.StatusCode != 200 {
		log.Println(res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromResponse(res)
	if err != nil {
		log.Println(err)
	}

	var message Post
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		op, _ := s.Attr("property")
		con, _ := s.Attr("content")

		if op == "og:image" {
			message.Thumbnail = con
		} else if op == "og:title" {
			splited := strings.Split(con, "-")
			title := strings.Join(splited[:1], "")
			message.Title = strings.TrimSpace(title)
		} else if op == "og:description" {
			message.Description = con
		} else if op == "og:updated_time" {
			message.Updated = con
		}
	})

	re := regexp.MustCompile("dcimg[0-9]")
	doc.Find(".writing_view_box").Find("img").Each(func(i int, s *goquery.Selection) {
		url, _ := s.Attr("src")
		url = re.ReplaceAllString(url, "images")
		url = strings.Replace(url, "co.kr", "com", 1)

		message.Images = append(message.Images, url)
	})

	return message
}

func Publish(pack Pack) {
	message, _ := json.Marshal(pack.Messages)
	// log.Println(string(message))
	log.Println(len(pack.Messages), "Message published")
	client.Publish("ib", message)
}

var client *redis.Client

func main() {
	client = redis.NewClient(&redis.Options{
		Addr:     "redis-10317.c16.us-east-1-3.ec2.cloud.redislabs.com:10317",
		Password: "WCkaZYzyhYR62p42VddCJba7Kn14vdvw",
		DB:       0,
	})

	if pong, err := client.Ping().Result(); err != nil {
		log.Fatal(err)
	} else {
		log.Println(pong)
	}

	for now := range time.Tick(time.Second * 3) {
		RequestList("https://gall.dcinside.com/board/lists?id=stream")
		log.Println("One cycle done", now)
	}
}
