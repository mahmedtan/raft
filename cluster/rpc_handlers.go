package cluster

import (
	context "context"
	"log"

	raftpb "github.com/mahmedtan/raft/proto"
	"google.golang.org/protobuf/proto"
)

// RequestVote handles incoming vote requests from other nodes in the cluster.
func (n *Node) RequestVote(_ context.Context, request *raftpb.RequestVoteRequest) (*raftpb.RequestVoteResponse, error) {

	hasOlderTerm := *request.CurrentTerm < n.persistentState.GetTerm()
	alreadyVotedForAnotherCandidate := *request.CurrentTerm == *n.persistentState.Term && n.persistentState.VotedFor != nil && *n.persistentState.VotedFor != *request.CandidateId

	if hasOlderTerm || alreadyVotedForAnotherCandidate {

		return &raftpb.RequestVoteResponse{
			VoteGranted: proto.Bool(false),
			CurrentTerm: proto.Int64(n.persistentState.GetTerm()),
		}, nil
	}

	var logTerm int64 = 0

	logEntries := n.persistentState.GetLogEntries()

	if len(logEntries) > 0 {
		logTerm = logEntries[len(logEntries)-1].GetTerm()
	}

	isLogOkay := *request.LastTerm > logTerm || (*request.LastTerm == logTerm && *request.LogLength >= int64(len(logEntries)))

	if isLogOkay {
		n.persistentState.Term = request.CurrentTerm
		n.persistentState.VotedFor = request.CandidateId
		n.SavePersistentState()

		return &raftpb.RequestVoteResponse{
			NodeId:      proto.Int64(*n.persistentState.NodeId),
			VoteGranted: proto.Bool(true),
			CurrentTerm: proto.Int64(n.persistentState.GetTerm()),
		}, nil

	} else {

		return &raftpb.RequestVoteResponse{
			NodeId:      proto.Int64(n.persistentState.GetNodeId()),
			VoteGranted: proto.Bool(false),
			CurrentTerm: proto.Int64(n.persistentState.GetTerm()),
		}, nil
	}

}

// StartVoteRequest starts the vote request process for the node to become a leader in the cluster.
func (node *Node) StartVoteRequest() {

	*node.persistentState.Term++

	node.role = Candidate
	node.persistentState.VotedFor = node.persistentState.NodeId
	node.votesRecvd = []int{int(node.persistentState.GetNodeId())}
	node.SavePersistentState()

	lastTerm := 0

	logEntries := node.persistentState.GetLogEntries()

	if len(logEntries) > 0 {
		lastTerm = len(logEntries) - 1
	}
	log.Printf("Node %v, Term %v: Starting vote request\n", node.persistentState.GetNodeId(), node.persistentState.GetTerm())

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
			CandidateId: proto.Int64(node.persistentState.GetNodeId()),
			CurrentTerm: proto.Int64(node.persistentState.GetTerm()),
			LogLength:   proto.Int64(int64(len(logEntries))),
			LastTerm:    proto.Int64(int64(lastTerm)),
		}

		reply, err := raftClient.RequestVote(context.Background(), req)

		if err != nil {
			log.Printf("Node %v: Error sending vote request to Node %v\n", node.persistentState.GetNodeId(), member)
			continue
		}

		if *reply.CurrentTerm > *node.persistentState.Term {
			log.Printf("Node %v, Term %v: Received higher term %v from Node %v, updating current term and resetting role\n", node.persistentState.GetNodeId(), node.persistentState.GetTerm(), *reply.CurrentTerm, member)

			node.persistentState.Term = reply.CurrentTerm
			node.persistentState.VotedFor = nil

			node.role = Follower
			node.votesRecvd = make([]int, 0)

			node.SavePersistentState()
			node.ResetTimer()

			return
		} else if node.role == Candidate && *reply.VoteGranted == true && node.persistentState.GetTerm() == *reply.CurrentTerm {
			log.Printf("Node %v, Term %v: Received vote from Node %v, total votes: %d\n", node.persistentState.GetNodeId(), node.persistentState.GetTerm(), member, len(node.votesRecvd))

			node.votesRecvd = append(node.votesRecvd, int(*reply.NodeId))

			if len(node.votesRecvd) > len(ClusterMembers)/2 {
				log.Printf("Node %v, Term %v: Received majority votes, becoming leader\n", node.persistentState.GetNodeId(), node.persistentState.GetTerm())
				node.role = Leader
				node.leaderId = int(node.persistentState.GetNodeId())
				node.ResetTimer()

				for _, member := range ClusterMembers {

					node.ackLen[member] = 0
					node.sentLen[member] = len(node.persistentState.GetLogEntries())
					node.ReplicateLog()
				}
			}
		}

	}

}

func (n *Node) ReplicateLog() {
	log.Printf("Node %v, Term %v: Replicating log entries to followers\n", n.persistentState.GetNodeId(), n.persistentState.GetTerm())
}
