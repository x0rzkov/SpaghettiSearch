package database

//package main

import (
	//"context"
	"encoding/json"
	//"fmt"
	//"github.com/apsdehal/go-logger"
	//"net/url"
	"strconv"
	"strings"
	"time"
)

/*
=============================== SCHEMA DEFINITION ==========================================

	Schema for inverted table for both body and title page schema:
		key	: DocId (type: uint32)
		value	: list of InvKeyword_value, where each contain the DocId and positions fo the word (type: InvKeyword_values, see InvKeyword_value)

	Schema for forward table forw[0]:
		key	: word (type: string)
		value	: wordId (type: uint32)

	Schema for forward table forw[1]:
		key	: wordId (type: uint32)
		value	: word (type: string)

	Schema for forward table forw[2]:
		key	: URL (type url.URL)
		value	: document info including DocId (type: DocInfo)

	Schema for forward table forw[3]:
		key:	: DocId (type: uint16)
		value	: URL (type: url.URL)

	Schema for forward table forw[4]:
		key	: index type (type: string)
		value	: biggest index value (type: uint32)

========================= MARSHAL AND UNMARSHALING =======================================

	Unless specified, all defined struct can be casted into array of bytes as below. Then the data can be passed for Set or any operation on the table object.

		byteArray, err := json.Marshal(any_struct_defined_in_this_file)


	To cast back into the desired data type, use Unmarshal operation

		byteArray, err := tableObject.Get(some_context, key_in_byteArray)
		var a desired_datatype
		err = json.Unmarshal(byteArray, &a)


	For url.URL data type, use command below to both encode it into array of byte and vice versa

		urlObject, err := url.Parse(url_in_string)
		byteArray, err := urlObject.MarshalBinary()

		tempUrl := &url.URL
		err := tempUrl.UnmarshalBinary(byteArray)
*/

// Each item in the a value of inverted table contains the DocId (type: uint16) and list of position of the word location in the document
type InvKeyword_value struct {
	DocId uint16   `json:"DocId"`
	Pos   []uint32 `json:"Pos"` // list of position of the word occuring in the document DocId
}

// InvKeyword_values contains slice of InvKeyword_value to support append operation
type InvKeyword_values []InvKeyword_value

// NOTE: Renamed after URL_value in the previous version
// DocInfo describes the document info and statistics, which serves as the value of forw[2] table (URL -> DocInfo)
type DocInfo struct {
	DocId         uint16           `json:"DocId"`
	Page_title	[]string	`json:"Page_title"`
	Mod_date      time.Time       `json:"Mod_date"`
	Page_size     uint32           `json:"Page_size"`
	Children      []uint16         `json:"Childrens"`
	Parents       []uint16         `json:"Parents"`
	Words_mapping map[uint32]uint32 `json:"Words_mapping"`
	//mapping for wordId to wordFrequency
}

func (u DocInfo) MarshalJSON() ([]byte, error) {
	basicDocInfo := struct {
		DocId         uint16           `json:"DocId"`
		Page_title	[]string	`json:"Page_title"`
		Mod_date      string          `json:"Mod_date"`
		Page_size     uint32           `json:"Page_size"`
		Children      []uint16         `json:"Childrens"`
		Parents       []uint16         `json:"Parents"`
		Words_mapping map[uint32]uint32 `json:"Words_mapping"`
	}{u.DocId, u.Page_title, u.Mod_date.Format(time.RFC1123), u.Page_size, u.Children, u.Parents, u.Words_mapping}

	return json.Marshal(basicDocInfo)
}

func (u *DocInfo) UnmarshalJSON(j []byte) error {
	var rawStrings map[string]interface{}

	err := json.Unmarshal(j, &rawStrings)
	if err != nil {
		return err
	}

	for k, v := range rawStrings {
		if v == nil {
			continue
		} else if strings.ToLower(k) == "docid" {
			u.DocId = uint16(v.(float64))
		} else if strings.ToLower(k) == "page_title" {
			u.Page_title = make([]string, len(v.([]interface{})))
			for k_, v_ := range v.([]interface{}) {
				u.Page_title[k_] = v_.(string)
			}
		} else if strings.ToLower(k) == "mod_date" {
			if u.Mod_date, err = time.Parse(time.RFC1123, v.(string)); err != nil {
				return err
			}
		} else if strings.ToLower(k) == "page_size" {
			u.Page_size = uint32(v.(float64))
		} else if strings.ToLower(k) == "children" {
			u.Children = make([]uint16, len(v.([]interface{})))
			for k_, v_ := range v.([]interface{}) {
				u.Children[k_] = uint16(v_.(float64))
			}
		} else if strings.ToLower(k) == "parents" {
			u.Parents = make([]uint16, len(v.([]interface{})))
			for k_, v_ := range v.([]interface{}) {
				u.Parents[k_] = uint16(v_.(float64))
			}
		} else if strings.ToLower(k) == "words_mapping" {
			u.Words_mapping = make(map[uint32]uint32)
			for k_, v_ := range v.(map[string]interface{}) {
				str, _ := strconv.ParseInt(k_, 0, 32)
				u.Words_mapping[uint32(str)] = uint32(v_.(float64))
			}
		}
	}

	return nil
}
/*
func main() {
	a := make(map[uint32]uint32)
	a[1]=10
	a[2]=20
	a[3]=30
	temp := DocInfo{
//		DocId:	12,
//		Page_title: []string{"asd","sdf"},
		Mod_date: time.Now(),
		Page_size: 32, 	
		Children: []uint16{1,2,3},
	//	Parents: []uint16{2,3,5},
		Words_mapping: a,
	}
		 
	b, _ := json.Marshal(temp)

	fmt.Println("after initialising", string(b))

	var un DocInfo
	json.Unmarshal(b, &un)
	fmt.Println("after unmarshaling", temp)

	dir := "../db_data/"
/*
	Name := make(map[uint32]uint32)
	Name[0] = 0
	Name[1] = 12
	Name[2] = 23
	Name[3] = 40
	tempdocinfo := DocInfo{
		DocId: 1,
		Mod_date: time.Now(),
		Page_size: 1,
		Children: []uint16{1,2,3},
		Parents: []uint16{1,4,6},
		Words_mapping:Name,
	}

	b1, _ := json.Marshal(tempdocinfo)
	fmt.Println("after initialising", string(b1))
	var tempb1 DocInfo

	json.Unmarshal(b1, &tempb1)
	fmt.Println("after unmarshaling", tempb1.Words_mapping)

	ctx, cancel := context.WithCancel(context.Background())
	log, _ := logger.New("test", 1)

	db, _ := NewBadgerDB(ctx, dir, log, false)
	defer db.Close(ctx, cancel)
	fmt.Println("BEFORE ADDITION")

	db.Delete(ctx, []byte("1"))
	db.Delete(ctx, []byte("2"))
	//db.Iterate(ctx)
	db.Set(ctx, []byte("1"), b)
	fmt.Println("AFTER ADDITION")
	//db.Iterate(ctx)
	c, _ := db.Get(ctx, []byte("1"))
	var tempd DocInfo
	json.Unmarshal(c, &tempd)
	fmt.Println("GET FROM DB", tempd.Page_title, tempd.Words_mapping, tempd.Page_size)
}
/*
	ctx, cancel := context.WithCancel(context.Background())
	log, _ := logger.New("test", 1)
	fmt.Println("using db_init...")
	inverted, forward, _ := DB_init(ctx, log)
	for _, bdb_i := range inverted {
		defer bdb_i.Close(ctx, cancel)
	}
	for _, bdb := range forward {
		defer bdb.Close(ctx, cancel)
	}

}*/
