package main

import (
	"encoding/json"
	"log"
	"net/http"
	"context"
	"github.com/apsdehal/go-logger"
	"github.com/gorilla/mux"
	"the-SearchEngine/parser"
	"math"
	db "the-SearchEngine/database"
	"sort"
	"sync"
	"encoding/hex"
	"github.com/dgraph-io/badger"
	"crypto/md5"
	"strings"
	"fmt"
)

// global declaration used in db
var forw []db.DB
var inv []db.DB
var ctx context.Context

func generateTermPipeline(listStr []string) <- chan string {
	out := make(chan string, len(listStr))
	for i := 0; i < len(listStr); i++ {
		out <- listStr[i]
	}
	close(out)
	return out
}

func generateAggrDocsPipeline(docRank map[string]Rank_term) <- chan Rank_result {
	out := make(chan Rank_result, len(docRank))
	for docHash, rank := range docRank {
		ret := Rank_result{DocHash: docHash, }
		for titleweight := range rank.TitleWeights {
			ret.TitleRank += float64(titleweight)
		}
		for bodyweight := range rank.BodyWeights {
			ret.BodyRank += float64(bodyweight)
		}

		out <- ret
	}
	close(out)
	return out
}

// several type for easier flow of channels
type Rank_term struct {
	TitleWeights	[]float32
	BodyWeights	[]float32
}

type Rank_result struct {
	DocHash		string
	TitleRank	float64
	BodyRank	float64
}

type Rank_combined struct {
	DocHash	string `json:"DocHash"` 
	Rank	float64 `json:"Rank"`
}

func (u Rank_combined) MarshalJSON() ([]byte, error) {
	b := struct {
		DocHash string `json:"DocHash"`
		Rank	float64 `json:"Rank"`
	}{u.DocHash, u.Rank}
	return json.Marshal(b)
}

func appendSort(data []Rank_combined, el Rank_combined) []Rank_combined {
	index := sort.Search(len(data), func(i int) bool { return data[i].Rank < el.Rank })
	data = append(data, Rank_combined{})
	copy(data[index+1:], data[index:])
	data[index] = el
	return data
}

func getFromInverted(ctx context.Context, termChan <-chan string, inv []db.DB) <-chan map[string]Rank_term {
	out := make(chan map[string]Rank_term)
	go func() {
		for term := range termChan {
			// get list of documents from both inverted tables
			var titleResult, bodyResult map[string][]float32
			if v, err := inv[0].Get(ctx, term); err != nil && err != badger.ErrKeyNotFound {
			
				panic(err)
			} else if v != nil {
				titleResult = v.(map[string][]float32)
			}

			if v, err := inv[1].Get(ctx, term); err != nil && err != badger.ErrKeyNotFound {
				panic(err)
			} else if v!= nil {
				bodyResult = v.(map[string][]float32)
			}

			// merge document retrieved from inverted tables
			ret := make(map[string]Rank_term)
			for docHash, listPos := range bodyResult {
				// first entry of the listPos is norm_tf*idf
				ret[docHash] = Rank_term{
					TitleWeights: nil,
					BodyWeights : []float32{listPos[0]},
				}
			}
			
			for docHash, listPos := range titleResult {
				tempVal := ret[docHash]
				// first entry of the listPos is norm_tf*idf
				tempVal.TitleWeights = []float32{listPos[0]}
				ret[docHash] = tempVal
			}
			
			out <- ret
		}
		close(out)
	}()
	return out
}
	
func fanInDocs(docsIn [] <-chan map[string]Rank_term) <- chan map[string]Rank_term {
	var wg sync.WaitGroup
	c := make(chan map[string]Rank_term)
	out := func(docs <-chan map[string]Rank_term) {
		defer wg.Done()
		for doc := range docs {
			c <- doc
		}
	}

	wg.Add(len(docsIn))
	for _, docs := range docsIn {
		go out(docs)
	}

	// close once all the output goroutines are done
	go func() {
		wg.Wait()
		close(c)
	}()
	
	return c
}

func fanInResult(docRankIn []<-chan Rank_combined) <- chan Rank_combined {
	var wg sync.WaitGroup
	c := make(chan Rank_combined)
	out := func(docs <-chan Rank_combined) {
		defer wg.Done()
		for doc := range docs {
			c <- doc
		}
	}

	wg.Add(len(docRankIn))
	for _, docRank := range docRankIn {
		go out(docRank)
	}

	// close once all the output goroutines are done
	go func() {
		wg.Wait()
		close(c)
	}()
	
	return c
}

func getMagnitudeAndPR(ctx context.Context, docs <- chan Rank_result, forw []db.DB, queryLength int) <- chan Rank_combined {
	out := make(chan Rank_combined)
	go func() {
		for doc := range docs {
			// get pagerank value
			var PR float64
			if tempVal, err := forw[3].Get(ctx, doc.DocHash); err != nil {
				panic(err)
			} else {
				PR = tempVal.(float64)
			}

			// get page magnitude for cossim normalisation
			var pageMagnitude map[string]float64
			if tempVal, err := forw[4].Get(ctx, doc.DocHash); err != nil {
				panic(err)
			} else {
				pageMagnitude = tempVal.(map[string]float64)
			}
			
			fmt.Println("DEBUG ", PR, pageMagnitude, doc.BodyRank, doc.TitleRank)
			// compute final rank
			queryMagnitude := math.Sqrt(float64(queryLength))
			doc.BodyRank /= (pageMagnitude["body"] * queryMagnitude)
			doc.TitleRank /= (pageMagnitude["title"] * queryMagnitude)
			

			out <- Rank_combined {
				DocHash : doc.DocHash, 
				Rank	: 0.4*PR + 0.4*doc.TitleRank + 0.2*doc.BodyRank,
			}
		}
		close(out)
	}()	
	return out
}		
							


func GetWebpages(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	query := params["terms"]
	// TODO: whether below is necessary
	// if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
	// 	panic(err)
	// }

	query = strings.Replace(query, "-", " ", -1)	
	log.Print("Querying terms:", query)
	queryTokenised := parser.Laundry(query)

	// convert to wordHash
	for i := 0; i < len(queryTokenised); i++ {
		tempHash := md5.Sum([]byte(queryTokenised[i]))
		queryTokenised[i] = hex.EncodeToString(tempHash[:])
	}

	// generate common channel with inputs
	termInChan := generateTermPipeline(queryTokenised)

	// fan-out to get term occurence from inverted tables
	numFanOut := int(math.Ceil(float64(len(queryTokenised)) * 0.75))
	termOutChan := [] (<-chan map[string]Rank_term){}
	for i := 0; i < numFanOut; i ++ {
		termOutChan = append(termOutChan, getFromInverted(ctx, termInChan, inv))
	}
	
	// fan-in the result and aggregate the result based on generator model
	// docsMatched has type map[string]Rank_term
	aggregatedDocs := make(map[string]Rank_term)
	for docsMatched := range fanInDocs(termOutChan) {
		for docHash, ranks := range docsMatched {
			val := aggregatedDocs[docHash]
			val.TitleWeights = append(val.TitleWeights, ranks.TitleWeights...)
			val.BodyWeights = append(val.BodyWeights, ranks.BodyWeights...)
			aggregatedDocs[docHash] = val
		}
	}	

	log.Print("DEBUG aggrDocs", aggregatedDocs)

	// common channel for inputs of final ranking calculation
	docsInChan := generateAggrDocsPipeline(aggregatedDocs)

	// fan-out to calculate final rank from PR and page magnitude
	numFanOut = int(math.Ceil(float64(len(aggregatedDocs))* 0.75))
	docsOutChan := [] (<-chan Rank_combined){}
	for i := 0; i < numFanOut; i++ {
		docsOutChan = append(docsOutChan, getMagnitudeAndPR(ctx, docsInChan, forw, len(queryTokenised)))
	}

	// fan-in final rank (generator pattern) and sort the result
	finalResult := make([]Rank_combined, len(aggregatedDocs))
	for docRank := range fanInResult(docsOutChan) {
		finalResult = appendSort(finalResult, docRank)
	}

	// return only top-50 document
	if len(finalResult) > 50 {
		json.NewEncoder(w).Encode(finalResult[:50])
	} else {
		json.NewEncoder(w).Encode(finalResult)
	}
}

func main() {
	// initialise db connection
	ctx, cancel := context.WithCancel(context.TODO())
	log_, _ := logger.New("test", 1)
	var err error
	inv, forw, err= db.DB_init(ctx, log_)
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
	log.Print("Server is on")
	router.HandleFunc("/query/{terms}", GetWebpages).Methods("GET")
	router.HandleFunc("/wordlist/{pre}", GetWordList).Methods("GET")
	log.Fatal(http.ListenAndServe(":8080", router))
}

func GetWordList(w http.ResponseWriter, r *http.Request) {
	log.Print("Getting word list...")

	pre := mux.Vars(r)["pre"]

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")

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
