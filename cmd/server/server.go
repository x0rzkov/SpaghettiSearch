package main

import (
	"context"
	"encoding/json"
	"github.com/apsdehal/go-logger"
	"github.com/gorilla/mux"
	db "github.com/nwihardjo/SpaghettiSearch/database"
	"github.com/nwihardjo/SpaghettiSearch/retrieval"
	"log"
	"net/http"
	"os"
	"sort"
	"time"
)

// global declaration used in db
var forw []db.DB
var inv []db.DB
var ctx context.Context

type request struct {
	Query string `json:"query"`
}

func setHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
}

func GetWebpages(w http.ResponseWriter, r *http.Request) {
	setHeader(w)
	if r.Method == "OPTIONS" {
		return
	}

	if r.Method == "POST" {
		var query request
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
			panic(err)
		}

		log.Print("Querying terms:", query)

		timer := time.Now()
		result := retrieval.Retrieve(query.Query, ctx, forw, inv)
		json.NewEncoder(w).Encode(result)

		log.Print("Query processed in ", time.Since(timer))
	}
}

func GetWordList(w http.ResponseWriter, r *http.Request) {
	log.Print("Getting word list...")

	pre := mux.Vars(r)["pre"]

	setHeader(w)

	tempT, err := inv[0].IterateInv(ctx, pre, forw[0])
	if err != nil {
		panic(err)
	}
	tempB, err := inv[1].IterateInv(ctx, pre, forw[0])
	if err != nil {
		panic(err)
	}
	merged_ := make(map[string]bool)
	for _, i := range tempT {
		merged_[i] = true
	}
	for _, i := range tempB {
		merged_[i] = true
	}
	tempT = []string{}
	tempB = []string{}
	var merged []string
	for k, _ := range merged_ {
		merged = append(merged, k)
		delete(merged_, k)
	}
	sort.Sort(sort.StringSlice(merged))
	json.NewEncoder(w).Encode(merged)
}

func main() {
	// bind to port for heroku deployment
	port := os.Getenv("PORT")
	if port == "" {
		log.Printf("$PORT must be set")
		port = "8080"
	}

	// initialise db connection
	ctx, cancel := context.WithCancel(context.TODO())
	log_, _ := logger.New("test", 1)
	var err error
	inv, forw, err = db.DB_init(ctx, log_)
	if err != nil {
		panic(err)
	}

	for _, bdb_i := range inv {
		defer bdb_i.Close(ctx, cancel)
	}
	for _, bdb := range forw {
		defer bdb.Close(ctx, cancel)
	}

	// initialise server
	router := mux.NewRouter()
	router.HandleFunc("/query", GetWebpages)
	router.HandleFunc("/query/{terms}", GetWebpages).Methods("GET")
	router.HandleFunc("/wordlist/{pre}", GetWordList).Methods("GET")

	// render react app
	buildPath := "./interface/build/"
	router.PathPrefix("/").Handler(http.FileServer(http.Dir(buildPath)))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static", http.FileServer(http.Dir(buildPath+"static/"))))

	// start the server
	log.Print("\n\nServer is running on port ", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
