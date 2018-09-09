package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	"github.com/thoas/go-funk"
)

type Item struct {
	ID     string `json: id`
	Tenant string `json: tenant`
}

var m = make(map[string]Item)

func post(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	decoder := json.NewDecoder(r.Body)

	var data Item
	err := decoder.Decode(&data)
	if err != nil {
		panic(err)
	}
	m[data.ID] = data
	log.Print(m)
	fmt.Fprintf(w, "Hello, %q", m)
}

func convertMapToSlice(map[string]Item) [][]string {
	pairs := [][]string{}
	for key, value := range m {
		pairs = append(pairs, []string{key, value.Tenant})
	}
	return pairs
}

func get(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tenant := ps[0].Value
	// convert map to slice to filter by tenant
	filteredMap := funk.Filter(convertMapToSlice(m), func(x []string) bool {
		return x[1] == tenant
	})
	json, err := json.Marshal(filteredMap)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(json)
}

func main() {
	router := httprouter.New()
	_, err := http.Get("http://" + os.Getenv("COORDINATOR_DOMAIN") + ":3000/newCounter/" + os.Getenv("PORT") + "/" + os.Getenv("HOSTNAME"))
	if err != nil {
		fmt.Println("Unable to reach the server.")
	}
	router.GET("/items/:tenant/count", get)
	router.POST("/items", post)
	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), router))
}
