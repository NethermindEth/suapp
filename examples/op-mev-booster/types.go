package main

import (
	"time"

	builderCapella "github.com/attestantio/go-builder-client/api/capella"
	"github.com/attestantio/go-eth2-client/spec/capella"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/holiman/uint256"
)

// Mev-Boost types
// opBidResp are entries in the bids cache for OP
type opBidResp struct {
	t        time.Time
	response OPBid
	bidInfo  bidInfo
	// relays   []RelayEntry
}

// bidInfo is used to store bid response fields for logging and validation
type bidInfo struct {
	blockHash   phase0.Hash32
	parentHash  phase0.Hash32
	pubkey      phase0.BLSPubKey
	blockNumber uint64
	txRoot      phase0.Root
	value       *uint256.Int
}

type OPBid struct {
	Value   *uint256.Int              `json:"value"`
	Payload *capella.ExecutionPayload `json:"payload"`
}

func translateResponse(bid builderCapella.SubmitBlockRequest) opBidResp {
	return opBidResp{
		t: time.Unix(int64(bid.ExecutionPayload.Timestamp), 0),
		response: OPBid{
			Value:   bid.Message.Value,
			Payload: bid.ExecutionPayload,
		},
		bidInfo: bidInfo{
			blockHash:   bid.Message.BlockHash,
			parentHash:  bid.Message.ParentHash,
			pubkey:      bid.Message.BuilderPubkey,
			blockNumber: bid.Message.Slot,
			value:       bid.Message.Value,
		},
	}
}
