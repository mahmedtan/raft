package cluster

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	raftpb "github.com/mahmedtan/raft/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func getRaftClientForNodeId(id int) (raftpb.RaftClient, error) {
	conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", getPortForNodeId(id)), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to node %d", id)
	}

	return raftpb.NewRaftClient(conn), nil
}

func getPersistentStateFileName(id int) string {
	return fmt.Sprintf("state/persistent_state_%d.pb", id)
}
func init() {
	rand.Seed(time.Now().UnixNano())
}

func getRandomTimeout() time.Duration {
	base := 1500 * time.Millisecond
	jitter := time.Duration(rand.Intn(1500)) * time.Millisecond // 1.5s–3.0s
	return base + jitter
}

func getPortForNodeId(id int) int {
	if id < 1 || id > 5 {
		log.Fatalf("Invalid Node ID: %d. Please choose an ID between 1 and 5.", id)
	}
	return BasePort + id
}

func logError(err error) {
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

}

var ClusterMembers = [...]int{1, 2, 3, 4, 5}

var HeartbeatInterval = 2 * time.Second
