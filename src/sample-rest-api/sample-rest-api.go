package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

const apiCallError = "Error in making API call to "
const lateResError = "Life is too short, make that API call faster!"

var sites = [3]string{"google", "duckduckgo", "twitter"}

type Configuration struct {
	Google  []string
	Twitter []string
}

var configuration Configuration

type seedInfo struct {
	url                  string
	chUrl, chText, chErr chan string
}

type Tuple struct {
	Url, Text, Error string
}

type SiteResult struct {
	Name    string
	Results []Tuple
}

type Reply struct {
	Query      string
	AllResults []SiteResult
}

type Result struct {
	query   string
	results map[string][]SiteData
}

type SiteData struct {
	url  string
	text string
}

func gAPiCall(url string, chUrl, chText, chErr chan string) {
	var link, snippet string
	resp, _ := http.Get(url)
	if resp.StatusCode != 200 {
		defer close(chErr)
		chErr <- apiCallError + sites[0]
		return
	}

	body, _ := ioutil.ReadAll(resp.Body)

	type Message struct {
		Items interface{}
	}
	var m Message
	err := json.Unmarshal(body, &m)
	defer func() {
		close(chUrl)
		close(chText)
		close(chErr)
	}()
	if err == nil {
		switch x := m.Items.(type) {
		case []interface{}:
			for _, e := range x {
				switch l2 := e.(type) {
				case map[string]interface{}:
					for k, val := range l2 {
						if k == "link" {
							switch l3 := val.(type) {
							case string:
								link = string(l3)
							}
						}
						if k == "snippet" {
							switch l4 := val.(type) {
							case string:
								snippet = string(l4)
							}
						}
					}
					for i := 0; i < 2; i++ {
						select {
						case chUrl <- link:
						case chText <- snippet:
						}
					}
				}
			}
		}
	}
}

func dApiCall(url string, chUrl, chText, chErr chan string) {
	var link, snippet string
	resp, _ := http.Get(url)
	if resp.StatusCode != 200 {
		defer close(chErr)
		chErr <- apiCallError + sites[1]
		return
	}
	body, _ := ioutil.ReadAll(resp.Body)

	type Message struct {
		RelatedTopics interface{}
	}
	var m Message
	err := json.Unmarshal(body, &m)
	defer func() {
		close(chUrl)
		close(chText)
		close(chErr)
	}()
	if err == nil {
		switch x := m.RelatedTopics.(type) {
		case []interface{}:
			for _, e := range x {
				switch l2 := e.(type) {
				case map[string]interface{}:
					for k, val := range l2 {
						if k == "FirstURL" {
							switch l3 := val.(type) {
							case string:
								link = string(l3)
							}
						}
						if k == "Text" {
							switch l4 := val.(type) {
							case string:
								snippet = string(l4)
							}
						}
					}
					for i := 0; i < 2; i++ {
						select {
						case chUrl <- link:
						case chText <- snippet:
						}
					}
				}
			}
		}
	}
}

func authenticateTwitter() bool {
	fmt.Println("New Bearer Token for Twitter!")
	client := &http.Client{}
	authUrl := "https://api.twitter.com/oauth2/token"
	bodyStr := []byte("grant_type=client_credentials")
	req, _ := http.NewRequest("POST", authUrl, bytes.NewBuffer(bodyStr))

	consumer_token := base64.URLEncoding.EncodeToString([]byte(configuration.Twitter[0] + ":" + configuration.Twitter[1]))
	auth_str := "Basic " + consumer_token

	req.Header.Add("Authorization", auth_str)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")

	resp, _ := client.Do(req)
	if resp.StatusCode == 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		type Token struct {
			Access_token string
		}
		var tok Token
		json.Unmarshal(body, &tok)

		configuration.Twitter[2] = tok.Access_token
		configs, _ := json.Marshal(configuration)
		os.Remove("conf.json")
		var f, _ = os.Create("conf.json")
		defer f.Close()
		w := bufio.NewWriter(f)
		fmt.Fprintf(w, "%v", string(configs))
		w.Flush()
		return true
	}
	return false
}

func tApiCall(url string, chUrl, chText, chErr chan string) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer "+configuration.Twitter[2])
	resp, err := client.Do(req)
	if resp.StatusCode != 200 {
		flag := authenticateTwitter()
		if !flag {
			defer close(chErr)
			chErr <- apiCallError + sites[2]
			return
		}
	}
	body, err := ioutil.ReadAll(resp.Body)
	type Message struct {
		Statuses interface{}
		Id       float64
	}
	var m Message
	err = json.Unmarshal(body, &m)
	defer func() {
		close(chUrl)
		close(chText)
		close(chErr)
	}()
	var link, snippet string
	if err == nil {
		switch x := m.Statuses.(type) {
		case []interface{}:
			for _, e := range x {
				switch l2 := e.(type) {
				case map[string]interface{}:
					for k, val := range l2 {
						if k == "source" {
							switch l3 := val.(type) {
							case string:
								link = string(l3)
							}
						}
						if k == "text" {
							switch l4 := val.(type) {
							case string:
								snippet = string(l4)
							}
						}
					}
					for i := 0; i < 2; i++ {
						select {
						case chUrl <- link:
						case chText <- snippet:
						}
					}
				}
			}
		}
	}
}

func makeApiCall(site, url, query string, chUrl, chText, chErr chan string) {
	var apiUrl string
	switch site {
	case sites[0]:
		apiUrl = url + "key=" + configuration.Google[0] + "&cx=" + configuration.Google[1] + "&q=" + query
		gAPiCall(apiUrl, chUrl, chText, chErr)
	case sites[1]:
		apiUrl = url + "q=" + query
		dApiCall(apiUrl, chUrl, chText, chErr)
	case sites[2]:
		apiUrl = url + "q=" + query
		tApiCall(apiUrl, chUrl, chText, chErr)
	}
}

func GetSearchResults(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	searchWord := params["searchWord"]
	var finalResult Reply
	if searchWord != "" {

		sitesInfo := []seedInfo{
			{"https://www.googleapis.com/customsearch/v1?", make(chan string), make(chan string), make(chan string)},
			{"http://api.duckduckgo.com/?format=json&", make(chan string), make(chan string), make(chan string)},
			{"https://api.twitter.com/1.1/search/tweets.json?", make(chan string), make(chan string), make(chan string)},
		}

		for i, siteInfo := range sitesInfo {
			go makeApiCall(sites[i], siteInfo.url, searchWord, siteInfo.chUrl, siteInfo.chText, siteInfo.chErr)
		}

		var siteResArr []SiteResult
		for i, siteInfo := range sitesInfo {
			var tupArr []Tuple

			select {
			case err := <-siteInfo.chErr:
				tupArr = append(tupArr, Tuple{Error: err})
			case val := <-siteInfo.chUrl:
				tupArr = append(tupArr, Tuple{Url: val, Text: <-siteInfo.chText})
				for val := range siteInfo.chUrl {
					tupArr = append(tupArr, Tuple{Url: val, Text: <-siteInfo.chText})
				}
			case <-time.After(1 * time.Second):
				tupArr = append(tupArr, Tuple{Error: lateResError})
			}

			siteResArr = append(siteResArr, SiteResult{sites[i], tupArr})
		}
		finalResult = Reply{searchWord, siteResArr}
	}
	json.NewEncoder(w).Encode(finalResult)
}

func main() {
	// Loading Authentication Config
	var file, _ = os.Open("conf.json")
	var decoder = json.NewDecoder(file)
	decoder.Decode(&configuration)
	file.Close()

	// REST API
	router := mux.NewRouter()
	router.HandleFunc("/{searchWord}", GetSearchResults).Methods("GET")
	log.Fatal(http.ListenAndServe(":8080", router))

}
