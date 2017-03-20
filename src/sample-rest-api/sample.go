package main

import (
	// "fmt"
	"net/http"
	"os"
	// "bufio"
	"encoding/json"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
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

func makeApiCall(site, url, query string, chUrl, chText, chErr chan string) {
	var apiUrl string
	switch site {
	case sites[0]:
		apiUrl = url + "key=" + configuration.Google[0] + "&cx=" + configuration.Google[1] + "&q=" + query
		gAPiCall(apiUrl, chUrl, chText, chErr)
	case sites[1]:
		apiUrl = url + "q=" + query
		dApiCall(apiUrl, chUrl, chText, chErr)
	}
}

func GetSearchResults(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	searchWord := params["searchWord"]
	var finalResult Reply
	if searchWord != "" {

		// seedUrls[sites[1]] = "https://api.twitter.com/1.1/search/tweets.json?"
		sitesInfo := []seedInfo{
			{"https://www.googleapis.com/customsearch/v1?", make(chan string), make(chan string), make(chan string)},
			{"http://api.duckduckgo.com/?format=json&", make(chan string), make(chan string), make(chan string)},
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

	// REST API
	router := mux.NewRouter()
	router.HandleFunc("/{searchWord}", GetSearchResults).Methods("GET")
	log.Fatal(http.ListenAndServe(":8080", router))

}
