package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sync"

	"github.com/julienschmidt/httprouter"
	"github.com/thoas/go-funk"
)

type Item struct {
	ID     string `json: ID`
	Tenant string `json: tenant`
}

var counterList = make([]string, 0)
var counterTurn = 0
var counterMux sync.Mutex

func incCounterTurn() {
	counterMux.Lock()
	defer counterMux.Unlock()
	counterTurn++
	if counterTurn >= len(counterList) {
		counterTurn = 0
	}
}

func sendToCounter(counterUel string, item Item, wg *sync.WaitGroup) {
	fmt.Println(counterUel)

	client := http.Client{}
	itemJSON, err := json.Marshal(item)
	if err != nil {
		wg.Done()
		panic(err)
	}
	req, err := http.NewRequest("POST", counterUel, bytes.NewBuffer(itemJSON))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		wg.Done()
		fmt.Println("Unable to reach the server.")
		return
	}
	fmt.Println(resp)
	wg.Done()
}

func getFromCounter(counterURL string, tenant string) ([][]string, error) {
	resp, err := http.Get(counterURL + "/" + tenant + "/count")
	if err != nil {
		fmt.Println("Unable to reach the server.")
		return nil, err
	}
	fmt.Println(resp)
	return encodeGETJson(resp.Body), nil

}

func encodeJSON(body io.ReadCloser) Item {
	decoder := json.NewDecoder(body)

	var data Item
	err := decoder.Decode(&data)
	if err != nil {
		panic(err)
	}
	return data
}

func encodeGETJson(body io.ReadCloser) [][]string {
	b, _ := ioutil.ReadAll(body)
	log.Println(string(b))
	var data [][]string
	err := json.Unmarshal(b, &data)
	if err != nil {
		panic(err)
	}
	return data
}

func post(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tmpCounter := counterTurn
	replicaCounterTurn := counterTurn + 1
	incCounterTurn()
	var wg sync.WaitGroup

	data := encodeJSON(r.Body)
	// send to counter turn
	wg.Add(1)
	go sendToCounter(counterList[tmpCounter], data, &wg)
	// send to next counter for replica
	if replicaCounterTurn == len(counterList) {
		replicaCounterTurn = 0
	}

	wg.Add(1)
	go sendToCounter(counterList[replicaCounterTurn], data, &wg)
	wg.Wait()

	log.Println(data.ID)
}

// retrive all element with tenant from all the counter
func spread(tenant string) ([]string, error) {
	sl := make([]string, 0)
	failCount := 0
	for _, URL := range counterList {
		values, err := getFromCounter(URL, tenant)
		if err != nil {
			failCount++
			continue
		}
		for _, element := range values {
			if element[0] != "" {
				sl = append(sl, element[0])
			}
		}
	}
	if failCount >= 2 {
		return nil, errors.New("multiple shards/partitions are down")
	}

	return sl, nil
}

func get(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tenant := ps[0].Value
	sl, err := spread(tenant)
	if err != nil {
		fmt.Fprintf(w, "%v", err)
		return
	}
	var m = make(map[string]string)
	for _, element := range sl {
		m[element] = element
	}
	fmt.Fprintf(w, "Hello, %v", len(m))
}

// dynamic add counters
func addNewCounter(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	port := ps[0].Value
	hostname := ps[1].Value
	url := "http://" + hostname + ":" + port + "/items"
	if !funk.Contains(counterList, url) {
		counterList = append(counterList, url)
	}
	fmt.Fprintf(w, "Port, %v added", len(counterList))
}

func main() {
	router := httprouter.New()
	router.GET("/items/:tenant/count", get)
	router.GET("/newCounter/:port/:hostname", addNewCounter)
	router.POST("/items", post)
	log.Fatal(http.ListenAndServe(":3000", router))
}
