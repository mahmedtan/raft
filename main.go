package main

import (
	"flag"
	"fmt"

	cluster "github.com/mahmedtan/raft/cluster"
)

var id = flag.Int("id", 0, "Node ID To Start With")

func main() {
	flag.Parse()

	if *id < 1 || *id > len(cluster.ClusterMembers) {
		fmt.Printf("Invalid Node ID: %v. Please choose an ID between 1 and %v.\n", *id, len(cluster.ClusterMembers))
		return
	}

	cluster.StartNode(*id)

}
