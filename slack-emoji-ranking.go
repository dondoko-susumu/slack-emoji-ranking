package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
)

var (
	apiUrl  string = "https://slack.com/api/conversations.list"
	apiUrl2 string = "https://slack.com/api/conversations.history"
	apiUrl3 string = "https://slack.com/api/chat.postMessage"
)

func main() {
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		log.Fatal("SLACK_TOKEN environment variable should be set")
	}

	values := url.Values{}
	values.Set("exclude_archived", "true")
	values.Add("limit", "1000")

	req, _ := http.NewRequest("GET", apiUrl+"?"+values.Encode(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Add("Content-type", "application/json")
	client := new(http.Client)
	resp, err := client.Do(req)

	if err != nil {
		fmt.Println(err)
		return
	}

	defer resp.Body.Close()

	channelList := &ChannelListResponse{}
	err = json.NewDecoder(resp.Body).Decode(channelList)

	if err != nil {
		fmt.Println(err)
		return
	}

	total := SafeCounter{v: make(map[string]int)}
	wg := sync.WaitGroup{}

	total.c = make(map[string]map[string]int)
	for _, c := range channelList.Channels {
		wg.Add(1)
		fmt.Println(c.ID, c.Name)
		total.makeChannelInc(c.Name)
		go GetChannelHistory(token, c.ID, c.Name, &total, &wg)
	}

	wg.Wait()

	orderTotal := List{}
	for k, v := range total.v {
		e := Entry{k, v}
		orderTotal = append(orderTotal, e)
	}

	sort.Sort(orderTotal)

	var text string
	text += fmt.Sprintf("Total\n")
	var i = 0
	for idx, entry := range orderTotal {
		text += fmt.Sprintf("%v :%s: %v\n", idx+1, entry.name, entry.value)
		i++
		if i == 20 {
			break
		}
	}
	for c, cv := range total.c {
		if len(cv) > 0 {
			var order = List{}
			for k, v := range cv {
				e := Entry{k, v}
				order = append(order, e)
			}
			sort.Sort(order)

			text += fmt.Sprintf("\n#%s\n", c)

			var i = 0
			for idx, entry := range order {
				text += fmt.Sprintf("%v :%s: %v\n", idx+1, entry.name, entry.value)
				i++
				if i == 5 {
					break
				}
			}
		}
	}
	fmt.Println(text)

	channel := os.Getenv("SLACK_SEND_CHANNEL")
	if channel == "" {
		channel = "general"
	}

	fmt.Println("Send Channel: " + channel)

	sendMessage(token, text, channel)
}

func GetChannelHistory(token string, channelID string, channelName string, total *SafeCounter, wg *sync.WaitGroup) error {
	defer wg.Done()

	values := url.Values{}
	values.Set("channel", channelID)
	values.Add("limit", "1000")

	req, _ := http.NewRequest("GET", apiUrl2+"?"+values.Encode(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Add("Content-type", "application/json")
	client := new(http.Client)
	resp, err := client.Do(req)

	if err != nil {
		fmt.Println(err)
		return err
	}

	defer resp.Body.Close()

	channelHistory := &ChannelHistoryResponse{}
	err = json.NewDecoder(resp.Body).Decode(channelHistory)

	if err != nil {
		fmt.Println(err)
		return err
	}

	for _, m := range channelHistory.Messages {
		//fmt.Println(m.User, m.Text)
		for _, r := range m.Reactions {
			//fmt.Println(r.Name, r.Count)
			total.Inc(r.Name, r.Count)
			total.ChannelInc(channelName, r.Name, r.Count)
		}
	}

	return nil
}

func sendMessage(token string, text string, channel string) {
	data := url.Values{}
	data.Set("channel", "#"+channel)
	data.Add("text", text)

	client := &http.Client{}
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s", apiUrl3), bytes.NewBufferString(data.Encode()))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, _ := client.Do(req)
	fmt.Println(resp.Status)
}

type Response struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type ChannelListResponse struct {
	Response
	Channels []Channel `json:"channels"`
}

type Channel struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	IsChannel   bool     `json:"is_channel"`
	Created     int      `json:"created"`
	Creator     string   `json:"creator"`
	Members     []string `json:"members"`
	LastRead    string   `json:"last_read"`
	UnreadCount int      `json:"unread_count"`
}

type ChannelHistoryResponse struct {
	Response
	Messages []Message `json:"messages"`
}

// Msg contains information about a slack message
type Message struct {
	// Basic Message
	Type      string `json:"type"`
	Channel   string `json:"channel"`
	User      string `json:"user"`
	Text      string `json:"text"`
	Timestamp string `json:"ts"`

	// reactions
	Reactions []Reaction `json:"reactions"`
}

type Reaction struct {
	// Basic Message
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Users []string `json:"users"`
}

type Entry struct {
	name  string
	value int
}
type List []Entry

func (l List) Len() int {
	return len(l)
}

func (l List) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l List) Less(i, j int) bool {
	if l[i].value == l[j].value {
		return (l[i].name < l[j].name)
	} else {
		return (l[i].value > l[j].value)
	}
}

// SafeCounter is safe to use concurrently.
type SafeCounter struct {
	v   map[string]int
	c   map[string]map[string]int
	mux sync.Mutex
}

// see http://qiita.com/daigo2010/items/d46975ad6decd8578c45

// Inc increments the counter for the given key.
func (c *SafeCounter) Inc(key string, cnt int) {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	c.v[key] = c.v[key] + cnt
	c.mux.Unlock()
}

func (c *SafeCounter) makeChannelInc(channel string) {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	c.c[channel] = make(map[string]int)
	c.mux.Unlock()
}

func (c *SafeCounter) ChannelInc(channel string, key string, cnt int) {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	c.c[channel][key] = c.c[channel][key] + cnt
	c.mux.Unlock()
}

// Value returns the current value of the counter for the given key.
func (c *SafeCounter) Value(key string) int {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	return c.v[key]
}

func (c *SafeCounter) ChannelValue(channel string, key string) int {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	return c.c[channel][key]
}
