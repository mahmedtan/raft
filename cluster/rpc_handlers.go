package cluster

import (
	context "context"
	"log"
	"time"

	raftpb "github.com/mahmedtan/raft/proto"
	"google.golang.org/protobuf/proto"
)

// RequestVote handles incoming vote requests from other nodes in the cluster.
func (n *Node) RequestVote(_ context.Context, request *raftpb.RequestVoteRequest) (*raftpb.RequestVoteResponse, error) {

	hasOlderTerm := request.GetTerm() < n.persistentState.GetCurrentTerm()
	alreadyVotedForAnotherCandidate := request.GetTerm() == n.persistentState.GetCurrentTerm() && n.persistentState.VotedFor != nil && n.persistentState.GetVotedFor() != request.GetCandidateId()

	if hasOlderTerm || alreadyVotedForAnotherCandidate {

		return &raftpb.RequestVoteResponse{
			VoteGranted: proto.Bool(false),
			Term:        n.persistentState.CurrentTerm,
		}, nil
	}

	n.MaybeBecomeFollower(*request.Term)

	isLogOkay := request.GetLastLogTerm() > n.GetLastLogTerm() || (request.GetLastLogTerm() == n.GetLastLogTerm() && request.GetLastLogIndex() >= n.GetLastLogIndex())

	if isLogOkay {
		n.persistentState.VotedFor = request.CandidateId
		n.SavePersistentState()
		n.ResetTimer()

		return &raftpb.RequestVoteResponse{
			VoteGranted: proto.Bool(true),
			Term:        n.persistentState.CurrentTerm,
		}, nil

	} else {
		return &raftpb.RequestVoteResponse{
			VoteGranted: proto.Bool(false),
			Term:        n.persistentState.CurrentTerm,
		}, nil
	}

}

// AppendEntries handles incoming log replication requests from the leader node.
func (n *Node) AppendEntries(_ context.Context, request *raftpb.AppendEntriesRequest) (*raftpb.AppendEntriesResponse, error) {
	return nil, nil
}

// StartElection starts the vote request process for the node to become a leader in the cluster.
func (node *Node) StartElection() {

	node.IncrementTerm()
	node.role = Candidate
	votesRecvd := []int64{node.persistentState.GetNodeId()}
	node.persistentState.VotedFor = node.persistentState.NodeId
	node.SavePersistentState()

	log.Printf("Node %v, Term %v: Starting vote request\n", node.persistentState.GetNodeId(), node.persistentState.GetCurrentTerm())

	for _, member := range ClusterMembers {

		if member == int(node.persistentState.GetNodeId()) {
			continue
		}

		raftClient, err := getRaftClientForNodeId(member)

		if err != nil {
			log.Printf("Node %v: Error getting Raft client for Node %v\n", node.persistentState.GetNodeId(), member)
			continue
		}

		req := &raftpb.RequestVoteRequest{
			CandidateId:  proto.Int64(node.persistentState.GetNodeId()),
			Term:         node.persistentState.CurrentTerm,
			LastLogTerm:  proto.Int64(node.GetLastLogTerm()),
			LastLogIndex: proto.Int64(node.GetLastLogIndex()),
		}

		reply, err := raftClient.RequestVote(context.Background(), req)

		if err != nil {
			log.Printf("Node %v: Error sending vote request to Node %v\n", node.persistentState.GetNodeId(), member)
			continue
		}

		node.MaybeBecomeFollower(*reply.Term)

		if node.role == Candidate && *reply.VoteGranted == true && *node.persistentState.CurrentTerm == *reply.Term {

			votesRecvd = append(votesRecvd, int64(member))

			log.Printf("Node %v, Term %v: Received vote from Node %v, total votes: %d\n", node.persistentState.GetNodeId(), node.persistentState.GetCurrentTerm(), member, len(votesRecvd))

			if len(votesRecvd) > (len(ClusterMembers) / 2) {
				log.Printf("Node %v, Term %v: Received majority votes, becoming leader\n", node.persistentState.GetNodeId(), node.persistentState.GetCurrentTerm())
				node.role = Leader
				node.leaderId = int(node.persistentState.GetNodeId())
				node.ResetTimer()

				// go node.StartHeartbeat()

				for _, member := range ClusterMembers {
					node.nextIndex[member] = len(node.persistentState.Log)
					node.matchIndex[member] = 0
				}

				node.ReplicateLog()

			}
		}

	}

}

func (n *Node) ReplicateLog() {
	log.Printf("Node %v, Term %v: Replicating log entries to followers\n", n.persistentState.GetNodeId(), n.persistentState.GetCurrentTerm())
}

func (n *Node) StartHeartbeat() {

	n.heartbeatTicker = time.NewTicker(HeartbeatInterval)

	log.Printf("Node %v, Term %v: Starting heartbeat\n", n.persistentState.GetNodeId(), n.persistentState.GetCurrentTerm())
	for n.role == Leader {
		<-n.heartbeatTicker.C

		for _, member := range ClusterMembers {

			if member == int(n.persistentState.GetNodeId()) {
				continue
			}

			n.ReplicateLog()
		}
	}

	log.Printf("Node %v, Term %v: Stopping heartbeat\n", n.persistentState.GetNodeId(), n.persistentState.GetCurrentTerm())

	n.heartbeatTicker.Stop()
}
