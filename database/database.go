package database

//package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/apsdehal/go-logger"
	"github.com/dgraph-io/badger"
	"log"
	"time"
)

const (
	// Default values are used. For garbage-collection purposes
	// TODO: to be fine-tuned
	badgerDiscardRatio = 0.5
	badgerGCInterval   = 10 * time.Minute
)

var (
	// BadgerAlertNamespace defines the alerts BadgerDB namespace
	BadgerAlertNamespace = []byte("alerts")
)

type (
	DB_Inverted interface {
		DB
		AppendValue(ctx context.Context, key []byte, appendedValue []byte) error
	}

	BadgerDB_Inverted struct {
		BadgerDB
	}
)

type (
	// TODO: add logger debug in each function
	DB interface {
		Get(ctx context.Context, key []byte) (value []byte, err error)
		Set(ctx context.Context, key []byte, value []byte) error
		Has(ctx context.Context, key []byte) (bool, error)
		Delete(ctx context.Context, key []byte) error
		Close(ctx context.Context, cancel context.CancelFunc) error
		// TODO: Iterate functionality to be implemented. Only printing atm
		Iterate(ctx context.Context) error
	}

	BadgerDB struct {
		db     *badger.DB
		logger *logger.Logger
	}
)

func DB_init(ctx context.Context, logger *logger.Logger) (inv []DB_Inverted, forw []DB, err error) {
	base_dir := "../db_data/"
	inverted_dir := []string{"invKeyword_body/", "invKeyword_title/"}
	forward_dir := []string{"Word_wordId/", "WordId_word/", "URL_docId/", "DocId_URL/", "Indexes/"}

	for _, v := range inverted_dir {
		temp, err := NewBadgerDB_Inverted(ctx, base_dir+v, logger)
		if err != nil {
			log.Fatal(err)
			return nil, nil, err
		}
		inv = append(inv, temp)
	}

	for _, v := range forward_dir {
		temp, err := NewBadgerDB(ctx, base_dir+v, logger)
		if err != nil {
			log.Fatal(err)
			return nil, nil, err
		}
		forw = append(forw, temp)
	}

	return inv, forw, nil
}

func NewBadgerDB_Inverted(ctx context.Context, dir string, logger *logger.Logger) (DB_Inverted, error) {
	opts := badger.DefaultOptions
	// set SyncWrites to False for performance increase but may cause loss of data
	opts.SyncWrites = true
	opts.Dir, opts.ValueDir = dir, dir

	badgerDB, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	bdb_i := &BadgerDB_Inverted{BadgerDB{badgerDB, logger}}

	// run garbage collection in advance
	go bdb_i.runGC(ctx)
	return bdb_i, nil
}

func NewBadgerDB(ctx context.Context, dir string, logger *logger.Logger) (DB, error) {
	opts := badger.DefaultOptions
	// set SyncWrites to False for performance increase but may cause loss of data
	opts.SyncWrites = true
	opts.Dir, opts.ValueDir = dir, dir

	badgerDB, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	bdb := &BadgerDB{
		db:     badgerDB,
		logger: logger,
	}

	// run garbage collection in advance
	go bdb.runGC(ctx)
	return bdb, nil
}

func (bdb_i *BadgerDB_Inverted) AppendValue(ctx context.Context, key []byte, appendedValue []byte) error {
	value, err := bdb_i.Get(ctx, key)
	if err != nil {
		log.Fatal(err)
		return err
	}

	var appendedValue_struct InvKeyword_value
	var tempValues InvKeyword_values
	err = json.Unmarshal(value, &tempValues)
	if err != nil {
		log.Fatal(err)
		return err
	}
	err = json.Unmarshal(appendedValue, &appendedValue_struct)
	if err != nil {
		log.Fatal(err)
		return err
	}

	tempValues = append(tempValues, appendedValue_struct)
	tempVal, err := json.Marshal(tempValues)
	if err != nil {
		log.Fatal(err)
		return err
	}

	// delete and set the new appended values
	// TODO: optimise the operation
	if err = bdb_i.Delete(ctx, key); err != nil {
		log.Fatal(err)
		return err
	}
	if err = bdb_i.Set(ctx, key, tempVal); err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func (bdb *BadgerDB) Get(ctx context.Context, key []byte) (value []byte, err error) {
	err = bdb.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)

		if err != nil {
			log.Fatal(err)
			return err
		}

		// value needed to be copied as it only lasts when the transaction is open
		err = item.Value(func(val []byte) error {
			value = append([]byte{}, val...)
			return nil
		})

		if err != nil {
			log.Fatal(err)
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return value, nil
}

func (bdb *BadgerDB) Set(ctx context.Context, key []byte, value []byte) error {
	err := bdb.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})

	if err != nil {
		bdb.logger.Debugf("Failed to set key %s: %v", key, value)
		return err
	}
	return nil
}

func (bdb *BadgerDB) Has(ctx context.Context, key []byte) (ok bool, err error) {
	_, err = bdb.Get(ctx, key)
	switch err {
	case badger.ErrKeyNotFound:
		ok, err = false, nil
	case nil:
		ok, err = true, nil
	}
	return
}

func (bdb *BadgerDB) Delete(ctx context.Context, key []byte) error {
	err := bdb.db.Update(func(txn *badger.Txn) error {
		err := txn.Delete(key)
		if err != nil {
			bdb.logger.Debugf("Failed to delete key: %v")
			return err
		}
		return nil
	})
	return err
}

func (bdb *BadgerDB) Close(ctx context.Context, cancel context.CancelFunc) error {
	// perform cancellation of the running process using context
	cancel()
	return bdb.db.Close()
}

func (bdb *BadgerDB) runGC(ctx context.Context) {
	ticker := time.NewTicker(badgerGCInterval)
	for {
		select {
		case <-ticker.C:
			err := bdb.db.RunValueLogGC(badgerDiscardRatio)
			if err != nil {
				if err == badger.ErrNoRewrite {
					bdb.logger.Debugf("No BadgerDB GC occured: %v", err)
				} else {
					bdb.logger.Errorf("Failed to GC BadgerDB: %v", err)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (bdb *BadgerDB) Iterate(ctx context.Context) error {
	err := bdb.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			err := item.Value(func(v []byte) error {
				fmt.Printf("\tkey=%s, value=%s\n", k, v)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

//func main() {
//	// testing db instantaneous
//	dir_ := "../db_data/"
//	log, err := logger.New("test", 1)
//	if err != nil { panic(err) }
//
//	ctx, cancel := context.WithCancel(context.Background())
//
//	db_temp, err := NewBadgerDB(ctx, dir_, log)
//	if err != nil { fmt.Println(err)}
//	defer db_temp.Close(ctx, cancel)
//
//	// db interface testing
//	fmt.Println("\tTESTING: Initial iteration")
//	db_temp.Iterate(ctx)
//	fmt.Println("\tTESTING: Setting values, and getting them back")
//	db_temp.Set(ctx, []byte("temp"), []byte("hi"))
//	value, err := db_temp.Get(ctx, []byte("temp"))
//	fmt.Printf("\tkey=temp, value=%s\n", value)
//	fmt.Println("\tTESTING: has functionality")
//	db_has, err := db_temp.Has(ctx, []byte("answer"))
//	fmt.Printf("\tdb_temp has anwer keys: %s\n", db_has)
//	fmt.Println("\tTESTING: deleting keys")
//	db_temp.Delete(ctx, []byte("temp"))
//	fmt.Println("\tlast iteration")
//	db_temp.Iterate(ctx)
//}