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
	apiUrl  string = "https://slack.com/api/channels.list"
	apiUrl2 string = "https://slack.com/api/channels.history"
	apiUrl3 string = "https://slack.com/api/chat.postMessage"
)

func main() {
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		log.Fatal("SLACK_TOKEN environment variable should be set")
	}

	values := url.Values{}
	values.Set("token", token)

	resp, err := http.Get(apiUrl + "?" + values.Encode())
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

	for _, c := range channelList.Channels {
		fmt.Println(c.ID, c.Name)
		_ = GetChannelHistory(token, c.ID, &total)
	}

	order := List{}
	for k, v := range total.v {
		e := Entry{k, v}
		order = append(order, e)
	}

	sort.Sort(order)
	var text string
	for idx, entry := range order {
		text += fmt.Sprintf("%v :%s: %v\n", idx+1, entry.name, entry.value)
	}

	fmt.Println(text)
	//sendMessage(token,text)
}

func GetChannelHistory(token string, channelID string, total *SafeCounter) error {
	values := url.Values{}
	values.Set("token", token)
	values.Add("channel", channelID)
	values.Add("count", "1000")

	resp, err := http.Get(apiUrl2 + "?" + values.Encode())
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
		}
	}

	return nil
}

func sendMessage(token string, text string) {
	data := url.Values{}
	data.Set("token", token)
	data.Add("channel", "#general")
	data.Add("text", text)

	client := &http.Client{}
	r, _ := http.NewRequest("POST", fmt.Sprintf("%s", apiUrl3), bytes.NewBufferString(data.Encode()))
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, _ := client.Do(r)
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
	mux sync.Mutex
}

// Inc increments the counter for the given key.
func (c *SafeCounter) Inc(key string, cnt int) {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	c.v[key] = c.v[key] + cnt
	c.mux.Unlock()
}

// Value returns the current value of the counter for the given key.
func (c *SafeCounter) Value(key string) int {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	return c.v[key]
}
