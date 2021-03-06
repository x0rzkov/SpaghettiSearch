package database

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"
)

/*
=============================== SCHEMA DEFINITION ==========================================
	Schema for inverted table for both body and title page schema:
		key	: wordHash (type: string)
		value	: map of docHash to list of positions (type: map[string][]uint32)
	Schema for forward table forw[0]:
		key	: wordHash (type: string)
		value	: word (type: string)
	Schema for forward table forw[1]:
		key	: docHash (type: string)
		value	: document info including the URL (type: DocInfo)
	Schema for forward table forw[2]:
		key	: docHash (type: string)
		value	: list of children's docHash (type: []string)
	Schema for forward table forw[3]:
		key	: docHash (type: string)
		value	: pageRank value (type: float64)
	Schema for forward table forw[4]:
		key	: docHash (type: string)
		value	: page magnitude (type: map[string]float64)
*/

// DocInfo describes the document info and statistics, which serves as the value of forw[2] table (URL -> DocInfo)
type DocInfo struct {
	Url        url.URL   `json:"Url"`
	Page_title []string  `json:"Page_title"`
	Mod_date   time.Time `json:"Mod_date"`
	Page_size  uint32    `json:"Page_size"`
	Children   []string  `json:"Children"`
	// mapping from parent Hash to anchor texts
	Parents map[string][]string `json:"Parents"`
	//mapping for wordHash to wordFrequency
	Words_mapping map[string]uint32 `json:"Words_mapping"`
}

// override json.Marshal to support marshalling of DocInfo type
func (u DocInfo) MarshalJSON() ([]byte, error) {
	basicDocInfo := struct {
		Url           string              `json:"Url"`
		Page_title    []string            `json:"Page_title"`
		Mod_date      string              `json:"Mod_date"`
		Page_size     uint32              `json:"Page_size"`
		Children      []string            `json:"Children"`
		Parents       map[string][]string `json:"Parents"`
		Words_mapping map[string]uint32   `json:"Words_mapping"`
	}{u.Url.String(), u.Page_title, u.Mod_date.Format(time.RFC1123), u.Page_size,
		u.Children, u.Parents, u.Words_mapping}

	return json.Marshal(basicDocInfo)
}

// override json.Unmarshal to uspport unmarshalling of DocInfo type
func (u *DocInfo) UnmarshalJSON(j []byte) error {
	var rawStrings map[string]interface{}

	err := json.Unmarshal(j, &rawStrings)
	if err != nil {
		return err
	}

	for k, v := range rawStrings {
		if v == nil {
			continue
		}
		switch strings.ToLower(k) {
		case "url":
			temp, err := url.Parse(v.(string))
			if err != nil {
				return err
			}
			u.Url = *temp
		case "page_title":
			u.Page_title = make([]string, len(v.([]interface{})))
			for k_, v_ := range v.([]interface{}) {
				u.Page_title[k_] = v_.(string)
			}
		case "mod_date":
			if u.Mod_date, err = time.Parse(time.RFC1123, v.(string)); err != nil {
				return err
			}
		case "page_size":
			u.Page_size = uint32(v.(float64))
		case "children":
			u.Children = make([]string, len(v.([]interface{})))
			for k_, v_ := range v.([]interface{}) {
				u.Children[k_] = v_.(string)
			}
		case "parents":
			u.Parents = make(map[string][]string)
			for k_, v_ := range v.(map[string]interface{}) {
				if v_ != nil {
					u.Parents[k_] = make([]string, len(v_.([]interface{})))
					for k2, v2 := range v_.([]interface{}) {
						u.Parents[k_][k2] = v2.(string)
					}
				} else {
					u.Parents[k_] = []string{}
				}
			}
		case "words_mapping":
			u.Words_mapping = make(map[string]uint32)
			for k_, v_ := range v.(map[string]interface{}) {
				u.Words_mapping[k_] = uint32(v_.(float64))
			}
		}
	}

	return nil
}

// helper function for type checking and conversion to support schema enforcement
// uses string approach for primitive data type to be converted to []byte
// uses json marshal function for complex / struct to be converted to []byte
// @return array of bytes, error
func checkMarshal(k interface{}, kType string, v interface{}, vType string) (key []byte, val []byte, err error) {
	err = nil

	// check the key type
	if kType != "" {
		switch kType {
		case "string":
			tempKey, ok := k.(string)
			if !ok {
				return nil, nil, ErrKeyTypeNotMatch
			}
			key = []byte(tempKey)
		default:
			return nil, nil, ErrKeyTypeNotFound
		}
	} else {
		key = nil
	}

	// don't need to check the value type if the key does not matched
	if err != nil {
		return nil, nil, ErrKeyTypeNotMatch
	}

	if vType != "" {
		switch vType {
		case "string":
			tempVal, ok := v.(string)
			if !ok {
				return nil, nil, ErrValTypeNotMatch
			}
			val = []byte(tempVal)
		case "[]string":
			tempVal, ok := v.([]string)
			if !ok {
				return nil, nil, ErrValTypeNotMatch
			}
			val, err = json.Marshal(tempVal)
		case "float64":
			tempVal, ok := v.(float64)
			if !ok {
				return nil, nil, ErrValTypeNotMatch
			}
			val, err = []byte(strconv.FormatFloat(tempVal, 'f', -1, 64)), nil
		case "map[string][]float32":
			tempVal, ok := v.(map[string][]float32)
			if !ok {
				return nil, nil, ErrValTypeNotMatch
			}
			val, err = json.Marshal(tempVal)
		case "map[string][]uint32":
			tempVal, ok := v.(map[string][]uint32)
			if !ok {
				return nil, nil, ErrValTypeNotMatch
			}
			val, err = json.Marshal(tempVal)
		case "map[string]uint32":
			tempVal, ok := v.(map[string]uint32)
			if !ok {
				return nil, nil, ErrValTypeNotMatch
			}
			val, err = json.Marshal(tempVal)
		case "map[string]float64":
			tempVal, ok := v.(map[string]float64)
			if !ok {
				return nil, nil, ErrValTypeNotMatch
			}
			val, err = json.Marshal(tempVal)
		case "DocInfo":
			tempVal, ok := v.(DocInfo)
			if !ok {
				return nil, nil, ErrValTypeNotMatch
			}
			val, err = json.Marshal(tempVal)
		default:
			return nil, nil, ErrValTypeNotFound
		}
	} else {
		val = nil
	}

	return
}

// helper function for type checking and conversion to support schema enforcement
func checkUnmarshal(v []byte, valType string) (val interface{}, err error) {
	switch valType {
	case "string":
		return string(v), nil
	case "[]string":
		var tempVal []string
		if err = json.Unmarshal(v, &tempVal); err != nil {
			return nil, err
		}
		return tempVal, nil
	case "float64":
		return strconv.ParseFloat(string(v), 64)
	case "map[string][]float32":
		tempVal := make(map[string][]float32)
		err = json.Unmarshal(v, &tempVal)
		if err != nil {
			return nil, err
		}
		return tempVal, nil
	case "map[string][]uint32":
		tempVal := make(map[string][]uint32)
		err = json.Unmarshal(v, &tempVal)
		if err != nil {
			return nil, err
		}
		return tempVal, nil
	case "map[string]uint32":
		tempVal := make(map[string]uint32)
		err = json.Unmarshal(v, &tempVal)
		if err != nil {
			return nil, err
		}
		return tempVal, nil
	case "map[string]float64":
		tempVal := make(map[string]float64)
		err = json.Unmarshal(v, &tempVal)
		if err != nil {
			return nil, err
		}
		return tempVal, nil
	case "DocInfo":
		var tempVal DocInfo
		err = json.Unmarshal(v, &tempVal)
		if err != nil {
			return nil, err
		}
		return tempVal, nil
	default:
		return nil, ErrValTypeNotFound
	}
}
