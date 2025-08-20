package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"google.golang.org/protobuf/proto"
)

var id = flag.Int("id", 0, "Node ID To Start With")

var ClusterMembers = [...]int{1, 2, 3, 4, 5}

func main() {
	flag.Parse()

	if *id < 1 || *id > len(ClusterMembers) {
		fmt.Printf("Invalid Node ID: %v. Please choose an ID between 1 and %v.\n", *id, len(ClusterMembers))
		return
	}

	node := Node{
		id:         *id,
		state:      Follower,
		leaderId:   0,
		votesRecvd: make([]int, 0),
	}

	timer := time.NewTimer(getRandomTimeout())

	var fileName = fmt.Sprintf("state/persistant_state_%d.pb", node.id)

	data, err := os.ReadFile(fileName)

	var persistantState *PersistentState

	if err != nil {

		if os.IsNotExist(err) {

			persistantState = &PersistentState{
				NodeId:     proto.Int64(int64(*id)),
				Term:       proto.Int64(0),
				LogEntries: make([]*LogEntry, 0),
				VotedFor:   nil,
			}

			data, err = proto.Marshal(persistantState)
			if err != nil {
				log.Fatalf("Error marshalling persistent state: %v", err)
			}

			os.WriteFile(fileName, data, 0644)
		}
	}

	fmt.Println(string(data))

	err = proto.Unmarshal(data, persistantState)

	if err != nil {
		log.Fatalf("Error unmarshalling persistent state: %v", err)
	}

	fmt.Println(persistantState)

	for {
		<-timer.C

		StartVoteRequest(&node)

		timer.Reset(getRandomTimeout())

	}

}

func getRandomTimeout() time.Duration {
	return time.Duration(rand.Intn(100)+1000) * time.Millisecond
}

func StartVoteRequest(node *Node) {
	// Logic to start a vote request
	log.Printf("Node %v: Starting vote request...\n", node.id)
	// Here you would typically send RPCs to other nodes
}
