package main

import (
	"flag"
	"fmt"
	"log"
)

var id = flag.Int("id", 0, "Node ID To Start With")

var ClusterMembers = [...]int{1, 2, 3, 4, 5}

func main() {
	flag.Parse()

	if *id < 1 || *id > len(ClusterMembers) {
		fmt.Printf("Invalid Node ID: %v. Please choose an ID between 1 and %v.\n", *id, len(ClusterMembers))
		return
	}

	node := NewNode(*id)

	fmt.Println(node.persistantState)

	for {
		<-node.timer.C
		log.Printf("Node %v: Timer expired, current state: %s\n", node.id, node.role)

		StartVoteRequest(node)

		node.ResetTimer()

	}

}

func StartVoteRequest(node *Node) {

	*node.persistantState.Term++
	node.SaveState()

	log.Printf("Node %v: Starting vote request \n", node.id)

}
