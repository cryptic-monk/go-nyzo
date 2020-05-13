package data_store

import (
	"database/sql"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"math/rand"
	"testing"
	"time"
)

func TestDb(t *testing.T) {
	var err error
	db, err := sql.Open("mysql", fmt.Sprintf("nyzo-test:this-is-a-test-yeah@tcp(127.0.0.1:3306)/nyzo-test"))
	if err != nil {
		fmt.Println(err)
		return
	}
	_, err = db.Exec(createTransactionsTableStatement)
	if err != nil {
		fmt.Println(err)
		return
	}
	stmt, err := db.Prepare(addTransactionStatement)
	if err != nil {
		fmt.Println(err)
		return
	}
	startTime := time.Now().UTC().UnixNano()
	// height
	for i := 0; i < 10000; i++ {
		// canonical
		for j := 0; j < 10; j++ {
			dummySender := make([]byte, 32)
			dummyRecipient := make([]byte, 32)
			dummyData := make([]byte, 32)
			dummySignature := make([]byte, 64)
			rand.Seed(time.Now().UTC().UnixNano())
			rand.Read(dummySender)
			rand.Read(dummyRecipient)
			rand.Read(dummyData)
			rand.Read(dummySignature)
			_, err = stmt.Exec(int64(i), int64(j), time.Now().UTC().UnixNano(), int8(blockchain_data.TransactionTypeStandard), dummySender, dummyRecipient, dummyData, int64(1), int64(1), int64(1), int64(1), dummySignature)
		}
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	fmt.Println("Time taken (s):")
	fmt.Println((time.Now().UTC().UnixNano() - startTime) / 1000000000)
}
