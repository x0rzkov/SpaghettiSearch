package indexer

import (
"the-SearchEngine/database"
"context"
"fmt"
"golang.org/x/net/html"
// "reflect"
"strconv"
//"github.com/apsdehal/go-logger"
"time"
"io/ioutil"
"bytes"
"sync"
"strings"
"os"
// "io"
"regexp"
"net/url"
"github.com/surgebase/porter2"
"github.com/dgraph-io/badger"
"encoding/json"
)

var docsDir = "docs/"
var stopWords = make(map[string]bool)
func isStopWord(s string) (isStop bool) {
	// create stopWords map if its 0
	if len(stopWords) == 0 {
		// import stopword file
		content, err := ioutil.ReadFile("./indexer/stopwords.txt")
		if err != nil {
			panic(err)
		}
		wordString := strings.Split(string(content), "\n")
		for _,word := range wordString {
			stopWords[word] = true
		}
	}
	isStop = stopWords[s]
	return
}
func laundry(s string) (c []string) {
	// remove all special characters
	regex := regexp.MustCompile("[^a-zA-Z0-9]")
	s = regex.ReplaceAllString(s, " ")
	// remove unnecessary spaces
	regex = regexp.MustCompile("[^\\s]+")
	words:= regex.FindAllString(s,-1)
	// loop through each word and clean them ~laundry time~
	for _,word := range words {
		cleaned := strings.TrimSpace(strings.ToLower(word))
		cleaned = porter2.Stem(cleaned)
		if !isStopWord(cleaned) {
			c = append(c, cleaned)
		}
	}
	return
}

func getWordInfo(words []string) (termFreq map[string]uint32,termPos map[string][]uint32){
	termFreq = make(map[string]uint32)
	termPos = make(map[string][]uint32)
	for pos, word := range words {
		termPos[word] = append(termPos[word], uint32(pos))
		termFreq[word] = termFreq[word]+1
	}
	return
}

func setInverted(ctx context.Context, word string, pos map[string][]uint32, nextDocID int, forward []database.DB, inverted database.DB_Inverted){
	// set InvKeyword_value
	invKeyVal := database.InvKeyword_value{uint16(nextDocID), pos[word]}
	mInvVal, err := json.Marshal(invKeyVal)
	if err != nil {
		panic(err)
	}
	// set InvKeyword_values
	invKeyVals := []database.InvKeyword_value{invKeyVal,}
	mInvVals, err := json.Marshal(invKeyVals)
	if err != nil {
		panic(err)
	}
	// Get wordID equivalent of current word
	wordID, err := forward[0].Get(ctx, []byte(word))
	// fmt.Println(nextWordID)
	// if there is no word to wordID mapping
	if err == badger.ErrKeyNotFound {
		// get latest wordID
			nextWordIDBytes, errNext := forward[4].Get(ctx, []byte("nextWordID"))
			if errNext == badger.ErrKeyNotFound {
				// masukkin 0 as nextWordID
				nextWordIDBytes = []byte(strconv.Itoa(0))
				forward[4].Set(ctx, []byte("nextWordID"), nextWordIDBytes)
			} else if errNext != nil {
				panic(errNext)
			}
			nextWordID, err := strconv.Atoi(string(nextWordIDBytes))
			if err != nil {
				panic(err)
			}
			// use nextWordID
			wordID = []byte(strconv.Itoa(nextWordID))
			// fmt.Println("new", newWordID)
			// forw[0] save word -> wordID
			forward[0].Set(ctx, []byte(word), wordID)
			// forw[1] save wordID -> word
			forward[1].Set(ctx, wordID, []byte(word))
			// update latest wordID
			forward[4].Set(ctx, []byte("nextWordID"), []byte(strconv.Itoa(nextWordID + 1)))
	} else if err != nil {
		panic(err)
	}
	// fmt.Println(word, wordID)
	hasWordID, err := inverted.Has(ctx, wordID)
	if err != nil {
		panic(err)
	}
	if hasWordID{
		// append both values are byte[]
		inverted.AppendValue(ctx, wordID, mInvVal)
	} else {
		// insert the list of inv
		inverted.Set(ctx, wordID, mInvVals)
	}
	return
}


func AddParent(currentURL string, parents []string,
	forw []database.DB, wgIndexer *sync.WaitGroup) {

	defer wgIndexer.Done()

	ctx, _ := context.WithCancel(context.TODO())

	docIdBytes, err := forw[2].Get(ctx, []byte(currentURL))
	if err != nil {
		panic(err)
	}
	tempdocinfoB, err := forw[3].Get(ctx, docIdBytes)
	if err != nil {
		panic(err)
	}
	var temp database.DocInfo
	err = temp.UnmarshalJSON(tempdocinfoB)
	if err != nil {
		panic(err)
	}
	for _, pURL := range parents {
		docIdPB, err := forw[2].Get(ctx, []byte(pURL))
		if err != nil {
			panic(err)
		}
		docIdP, err := strconv.Atoi(string(docIdPB))
		if err != nil {
			panic(err)
		}
		temp.Parents = append(temp.Parents, uint16(docIdP))
	}
	newDocInfoBytes, err := temp.MarshalJSON()
	if err != nil {
		panic(err)
	}
	err = forw[3].Set(ctx, docIdBytes, newDocInfoBytes)
	if err != nil {
		panic(err)
	}

}

func Index(doc []byte, urlString string,
	lastModified time.Time, ps string, mutex *sync.Mutex,
	inverted []database.DB_Inverted, forward []database.DB,
	parentURL string, children []string) {

	var title string
	var prevToken string
	var words []string
	var cleaned string

	/* parentURL == "" means nil
	if parentURL == "" {
		handle parentURL as nil
	}
	*/

	ctx, _ := context.WithCancel(context.TODO())

	// Get Last Modified from DB
	URL, err := url.Parse(urlString)
	if err != nil {
		panic(err)
	}
	URLBytes, errMarshal := URL.MarshalBinary()
	if errMarshal != nil {
		panic(errMarshal)
	}
	fmt.Println("Indexing", URL.String())

	//BEGIN LOCK//
	mutex.Lock()
	nextDocIDBytes, errNext := forward[4].Get(ctx, []byte("nextDocID"))
	if errNext == badger.ErrKeyNotFound {
		// masukkin 0 as nextDocID
		// fmt.Println("initialize next DocID")
		forward[4].Set(ctx, []byte("nextDocID"), []byte(strconv.Itoa(0)))
		nextDocIDBytes = []byte(strconv.Itoa(0))
	} else if errNext != nil {
		panic(errNext)
	}
	nextDocID, err := strconv.Atoi(string(nextDocIDBytes))
	if err != nil {
		panic(err)
	}

	// check if current doc has an ID
	docIDBytes, err := forward[2].Get(ctx, URLBytes)
	if err == badger.ErrKeyNotFound {
		// set docID
		docIDBytes = nextDocIDBytes
		// add this doc to forw[2]
		forward[2].Set(ctx, URLBytes, docIDBytes)
		forward[4].Set(ctx, []byte("nextDocID"), []byte(strconv.Itoa(nextDocID + 1)))
	}
	docID, err := strconv.Atoi(string(docIDBytes))
	if err != nil {
		panic(err)
	}
	//Tokenize document
	tokenizer := html.NewTokenizer(bytes.NewReader(doc))
	for {
		tokenType := tokenizer.Next()
		// end of file or html error
		if tokenType == html.ErrorToken {
			break
		}
		token := tokenizer.Token()
		switch tokenType {
			case html.StartTagToken:
			if token.Data == "title" {
			tokenizer.Next()
			title = strings.TrimSpace(tokenizer.Token().Data)
		}
		prevToken = token.Data
		break
		case html.TextToken:
			cleaned = strings.TrimSpace(token.Data)
			if prevToken != "script" && prevToken != "a" && prevToken != "style" && cleaned != ""{
				words = append(words, cleaned)
			}
		break
		}
	}
	// tokenize terms in title and body
	cleanTitle := laundry(title)
	cleanBody := laundry(strings.Join(words, " "))
	// get words info
	_, posTitle := getWordInfo(cleanTitle)
	freqBody, posBody := getWordInfo(cleanBody)
	for _, word := range cleanTitle {
		// save from title wordID -> [{DocID, Pos}]
		setInverted(ctx, word, posTitle, docID, forward, inverted[0])
	}
	for _, word := range cleanBody {
		// save from body wordID-> [{DocID, Pos}]
		setInverted(ctx, word, posBody, docID, forward, inverted[1])
	}
	fmt.Println("HEL")
	var kids []uint16
	// get the URL mapping of each child
	if children != nil {
		for _,child := range children {
			// fmt.Println(child)
			childURL, err := url.Parse(child)
			if err != nil{
				panic(err)
			}
			mChildURL, errMarshal := childURL.MarshalBinary()
			if errMarshal != nil {
				panic(errMarshal)
			}
			childIDBytes, err := forward[2].Get(ctx, mChildURL)
			if err == badger.ErrKeyNotFound {
				// get the next doc ID
				nextDocIDBytes, errNext := forward[4].Get(ctx, []byte("nextDocID"))
				if errNext != nil {
					panic(errNext)
				}
				nextDocID, err := strconv.Atoi(string(nextDocIDBytes))
				if err != nil {
					panic(err)
				}
				docInfoC := database.DocInfo{
					*childURL,
					nil,
					time.Now(),
					0,
					nil,
					[]uint16{uint16(nextDocID),},
					nil,
				}
				docInfoBytes, err := json.Marshal(docInfoC)
				if err != nil {
					panic(err)
				}
				// child is not inserted into URL->DocID
				forward[2].Set(ctx, mChildURL, nextDocIDBytes)
				forward[3].Set(ctx, nextDocIDBytes, docInfoBytes)
				// set childID
				childIDBytes = nextDocIDBytes
				// update nextDocID
				forward[4].Set(ctx, []byte("nextDocID"), []byte(strconv.Itoa(nextDocID + 1)))
			}
			childID, err := strconv.Atoi(string(childIDBytes))
			if err != nil {
				panic(err)
			}
			// fmt.Println(childID)
			kids = append(kids, uint16(childID))
		}
		fmt.Println(kids)
	}
	// forw[2] save URL -> DocInfo
	// URL to the marshalling stuff
	// parse title
	pageTitle := strings.Fields(title)
	var pageSize int
	if ps == "" {
		pageSize = len(doc)
	} else {
		pageSize, err = strconv.Atoi(ps)
		if err != nil {
			panic(err)
		}
	}

	wordMapping := make(map[uint32]uint32)
	for word, _ := range freqBody {
		wordIDBytes, err := forward[0].Get(ctx, []byte(word))
		if err != nil {
			panic(err)
		}
		wordID, err := strconv.Atoi(string(wordIDBytes))
		if err != nil {
			fmt.Println(word)
			panic(err)
		}
		wordMapping[uint32(wordID)] = freqBody[word]
	}
	fmt.Println("final stretch...")
	pageInfo := database.DocInfo{*URL, pageTitle, lastModified, uint32(pageSize), kids, nil, wordMapping}
	// marshal pageInfo
	mPageInfo, err := pageInfo.MarshalJSON()
	if err != nil {
		panic(err)
	}
	// insert into forward 3
	forward[3].Set(ctx, docIDBytes, mPageInfo)
	fmt.Println("more final than final")
	// update forward table for DocID and its corresponding URL
	// forward[3].Set(ctx, []byte(strconv.Itoa(nextDocID)), URLBytes)
	mutex.Unlock()
	//END LOCK//
	// type DocInfo struct {
	// 	DocId         uint16            `json:"DocId"`
	// 	Page_title    []string          `json:"Page_title"`
	// 	Mod_date      time.Time         `json:"Mod_date"`
	// 	Page_size     uint32            `json:"Page_size"`
	// 	Children      []uint16          `json:"Childrens"`
	// 	Parents       []uint16          `json:"Parents"`
	// 	Words_mapping map[uint32]uint32 `json:"Words_mapping"`
	// 	//mapping for wordId to wordFrequency
	// }

	// Save to file
	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
	os.Mkdir(docsDir, 0755)
	}
	err = ioutil.WriteFile(docsDir + strconv.Itoa(nextDocID), doc, 0644)
	if err != nil {
	panic(err)
	}
	fmt.Println("i RETURNED")
}
