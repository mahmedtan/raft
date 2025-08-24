package main

import (
	"context"
	"flag"
	"fmt"

	cluster "github.com/mahmedtan/raft/cluster"
	raftpb "github.com/mahmedtan/raft/proto"
)

var id = flag.Int("id", 0, "Node ID To Start With")
var message = flag.String("msg", "", "Message To Broadcast (Only for Leader)")

func main() {
	flag.Parse()

	if *id < 1 || *id > len(cluster.ClusterMembers) {
		fmt.Printf("Invalid Node ID: %v. Please choose an ID between 1 and %v.\n", *id, len(cluster.ClusterMembers))
		return
	}

	if message != nil && *message != "" {
		raftClient, err := cluster.GetRaftClientForNodeId(*id)
		if err != nil {
			fmt.Printf("Failed to connect to node %d: %v\n", *id, err)
			return
		}

		reply, err := raftClient.BroadcastMessage(context.Background(), &raftpb.BroadcastMessageRequest{
			Message: message,
		})
		if err != nil {
			fmt.Printf("Failed to broadcast message: %v\n", err)
			return
		}

		fmt.Printf("Broadcast Response from Node %d: Success=%v\n", *id, reply.GetSuccess())
	} else {

		cluster.StartNode(*id)
	}

}
