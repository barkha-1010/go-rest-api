package main

import (
	"fmt"
	"net/http"
	// "os"
	// "bufio"
	"encoding/json"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"time"
)

type Result struct {
	query   string
	results map[string][]SiteData
}

type SiteData struct {
	url  string
	text string
}

func gAPiCall(url string, chUrl, chText chan string) {
	var link, snippet string
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error in making API call to " + sites[0])
		// return
	}
	// resp.Status
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error in reading result from API call to " + sites[0])
		// return
	}
	type Message struct {
		Items interface{}
	}
	var m Message
	err = json.Unmarshal(body, &m)
	defer func() {
		close(chUrl)
		close(chText)
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

func dApiCall(url string, chUrl, chText chan string) {
	var link, snippet string
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error in making API call to " + sites[1])
		// return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error in reading result from API call to " + sites[1])
		// return
	}
	type Message struct {
		RelatedTopics interface{}
	}
	var m Message
	err = json.Unmarshal(body, &m)
	defer func() {
		close(chUrl)
		close(chText)
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

func makeApiCall(site, url, query string, chUrl, chText chan string) {
	var apiUrl string
	switch site {
	case sites[0]:
		apiUrl = url + "key=" + gCreds.key + "&cx=" + gCreds.engine + "&q=" + query
		gAPiCall(apiUrl, chUrl, chText)
	case sites[1]:
		apiUrl = url + "q=" + query
		dApiCall(apiUrl, chUrl, chText)
	}
}

type LoginCreds struct {
	access_token        string
	access_token_secret string
	key                 string
	engine              string
}

type seedInfo struct {
	url    string
	chUrl  chan string
	chText chan string
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

var sites = [3]string{"google", "duckduckgo", "twitter"}

// add authentication credentials here
var gCreds LoginCreds = LoginCreds{key: "", engine: ""}
var tCreds LoginCreds = LoginCreds{access_token: "", access_token_secret: ""}

func GetSearchResults(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	searchWord := params["searchWord"]
	var finalResult Reply
	if searchWord != "" {

		// seedUrls[sites[1]] = "https://api.twitter.com/1.1/search/tweets.json?"
		sitesInfo := []seedInfo{
			{"https://www.googleapis.com/customsearch/v1?", make(chan string), make(chan string)},
			{"http://api.duckduckgo.com/?format=json&", make(chan string), make(chan string)},
		}

		for i, siteInfo := range sitesInfo {
			go makeApiCall(sites[i], siteInfo.url, searchWord, siteInfo.chUrl, siteInfo.chText)
		}

		var siteResArr []SiteResult
		for i, siteInfo := range sitesInfo {
			var tupArr []Tuple

			select {
			case val := <-siteInfo.chUrl:
				tupArr = append(tupArr, Tuple{Url: val, Text: <-siteInfo.chText})
				for val := range siteInfo.chUrl {
					tupArr = append(tupArr, Tuple{Url: val, Text: <-siteInfo.chText})
				}
			case <-time.After(1 * time.Second):
				tupArr = append(tupArr, Tuple{Error: "Result was late!"})
			}

			siteResArr = append(siteResArr, SiteResult{sites[i], tupArr})
		}
		finalResult = Reply{searchWord, siteResArr}
	}
	json.NewEncoder(w).Encode(finalResult)
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/{searchWord}", GetSearchResults).Methods("GET")
	log.Fatal(http.ListenAndServe(":8080", router))

}
