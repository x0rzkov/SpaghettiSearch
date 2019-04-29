package main

import (
	"context"
	"encoding/json"
	"github.com/apsdehal/go-logger"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"sort"
	"strings"
	db "the-SearchEngine/database"
	"the-SearchEngine/retrieval"
	"time"
)

// global declaration used in db
var forw []db.DB
var inv []db.DB
var ctx context.Context

func setHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
}

func GetWebpages(w http.ResponseWriter, r *http.Request) {
	setHeader(w)
	//---------------- QUERY PARSING ----------------//

	params := mux.Vars(r)
	query := params["terms"]
	// if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
	// 	panic(err)
	// }

	query = strings.Replace(query, "-", " ", -1)
	log.Print("Querying terms:", query)
	timer := time.Now()

	result := retrieval.Retrieve(query, ctx, forw, inv)

	json.NewEncoder(w).Encode(result)

	log.Print("Query processed in ", time.Since(timer))
}

func main() {
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

	// start server
	router := mux.NewRouter()
	log.Print("Server is running")
	router.HandleFunc("/query/{terms}", GetWebpages).Methods("GET")
	router.HandleFunc("/wordlist/{pre}", GetWordList).Methods("GET")
	log.Fatal(http.ListenAndServe(":8080", router))
}
