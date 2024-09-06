package generator

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var (
	userIDCounter int64 = 1000000000000
	userIDPool    []int
	userIDMutex   sync.Mutex
)

type UserBehaviorRecord struct {
	USER_ID   int    `dynamodbav:"user_id"`
	CLIENT_TS int64  `dynamodbav:"client_ts"`
	ACTION    string `dynamodbav:"action"`
	TS_RANK   int    `dynamodbav:"ts_rank"`
}

func init() {
	userIDPool = make([]int, 10000)
	for i := 0; i < 10000; i++ {
		userIDPool[i] = int(userIDCounter) + i
	}

	rand.Shuffle(len(userIDPool), func(i, j int) {
		userIDPool[i], userIDPool[j] = userIDPool[j], userIDPool[i]
	})
}

func CreateUserBehaviorRecord() UserBehaviorRecord {
	return UserBehaviorRecord{
		USER_ID:   getRandomUserID(),
		CLIENT_TS: time.Now().UnixNano() / int64(time.Millisecond),
		ACTION:    randomAction(),
		TS_RANK:   0,
	}
}

func getRandomUserID() int {
	userIDMutex.Lock()
	defer userIDMutex.Unlock()

	if len(userIDPool) == 0 {

		for i := 0; i < 10000; i++ {
			userIDPool = append(userIDPool, int(userIDCounter)+i)
		}
		rand.Shuffle(len(userIDPool), func(i, j int) {
			userIDPool[i], userIDPool[j] = userIDPool[j], userIDPool[i]
		})
	}

	userID := userIDPool[len(userIDPool)-1]
	userIDPool = userIDPool[:len(userIDPool)-1]
	return userID
}

func randomAction() string {
	return fmt.Sprintf("action-%04d", rand.Intn(2001))
}

func GenerateUserBehaviorRecords(totalRecords, recordsPerSecond int, recordChan chan<- UserBehaviorRecord) {
	batchSize := recordsPerSecond
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for i := 0; i < totalRecords; i += batchSize {
		batch := make([]UserBehaviorRecord, 0, batchSize)
		for j := 0; j < batchSize && i+j < totalRecords; j++ {
			batch = append(batch, CreateUserBehaviorRecord())
		}

		for _, record := range batch {
			recordChan <- record
		}

		<-ticker.C
	}
}
