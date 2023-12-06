// SPDX-License-Identifier: UNLICENSED

pragma solidity ^0.8.8;

import "../../suave-geth/suave/sol/libraries/Suave.sol";


struct EgpBidPair {
	uint64 egp; // in wei, beware overflow
	Suave.BidId bidId;
}

contract MevBoost {
	event BidEvent(
		Suave.BidId bidId,
		uint64 decryptionCondition,
		address[] allowedPeekers
	);

    event BuilderBoostBidEvent(
        Suave.BidId bidId,
        bytes builderBid
    );

    function buildBlock(Suave.BuildBlockArgs memory blockArgs, uint64 blockHeight) public returns (bytes memory) {
        require(Suave.isConfidential());

        Suave.Bid[] memory allBids = Suave.fetchBids(blockHeight, "default:v0:ethBundles");
        if (allBids.length == 0) {
            revert Suave.PeekerReverted(address(this), "no bids");
        }

        EgpBidPair[] memory bidsByEGP = new EgpBidPair[](allBids.length);
        for (uint i = 0; i < allBids.length; i++) {
            bytes memory simResults = Suave.confidentialRetrieve(allBids[i].id, "default:v0:ethBundleSimResults");
            uint64 egp = abi.decode(simResults, (uint64));
            bidsByEGP[i] = EgpBidPair(egp, allBids[i].id);
        }

        bidsByEGP = sortBundles(bidsByEGP);

        Suave.BidId[] memory allBidIds = new Suave.BidId[](allBids.length);
        for (uint i = 0; i < bidsByEGP.length; i++) {
            allBidIds[i] = bidsByEGP[i].bidId;
        }

        (Suave.Bid memory blockBid, bytes memory builderBid) = this.doBuild(blockArgs, blockHeight, allBidIds, "");
        emit BuilderBoostBidEvent(blockBid.id, builderBid);
        emit BidEvent(blockBid.id, blockBid.decryptionCondition, blockBid.allowedPeekers);
        return bytes.concat(this.emitBuilderBidAndBid.selector, abi.encode(blockBid, builderBid));
    }

    function sortBundles(EgpBidPair[] memory bidsByEGP) internal pure returns (EgpBidPair[] memory) {
        // Bubble sort, cause why not
        uint n = bidsByEGP.length;
        for (uint i = 0; i < n - 1; i++) {
            for (uint j = i + 1; j < n; j++) {
                if (bidsByEGP[i].egp < bidsByEGP[j].egp) {
                    EgpBidPair memory temp = bidsByEGP[i];
                    bidsByEGP[i] = bidsByEGP[j];
                    bidsByEGP[j] = temp;
                }
            }
        }
        return bidsByEGP;
    }

    function doBuild(Suave.BuildBlockArgs memory blockArgs,
                     uint64 blockHeight,
                     Suave.BidId[] memory bids,
                     string memory namespace) public view returns (Suave.Bid memory, bytes memory) {

        address[] memory allowedPeekers = new address[](3);
        allowedPeekers[0] = address(this);
        allowedPeekers[1] = Suave.BUILD_ETH_BLOCK;

        Suave.Bid memory blockBid = Suave.newBid(blockHeight, allowedPeekers, allowedPeekers, "default:v0:mergedBids");
        Suave.confidentialStore(blockBid.id, "default:v0:mergedBids", abi.encode(bids));

        (bytes memory builderBid, bytes memory payload) = Suave.buildEthBlock(blockArgs, blockBid.id, namespace);
        Suave.confidentialStore(blockBid.id, "default:v0:builderPayload", payload); // only through this.unlock

        return (blockBid, builderBid);
    }

    function emitBuilderBidAndBid(Suave.Bid memory bid, bytes memory builderBid) public returns (Suave.Bid memory, bytes memory) {
        emit BuilderBoostBidEvent(bid.id, builderBid);
        emit BidEvent(bid.id, bid.decryptionCondition, bid.allowedPeekers);
        return (bid, builderBid);
    }

    function getBlock(Suave.BidId bidId) public view returns (bytes memory) {
        require(Suave.isConfidential());

        // TODO: Access control to the current proposer only.
        bytes memory payload = Suave.confidentialRetrieve(bidId, "default:v0:builderPayload");
        return payload;
    }
}

// Missing contract from bids.sol
contract BundleBidContract {
    event BidEvent(
        Suave.BidId bidId,
        uint64 decryptionCondition,
        address[] allowedPeekers
    );

    function newBid(uint64 decryptionCondition, address[] memory bidAllowedPeekers, address[] memory bidAllowedStores) external payable returns (bytes memory) {
        require(Suave.isConfidential());

        bytes memory bundleData = this.fetchBidConfidentialBundleData();

        uint64 egp = Suave.simulateBundle(bundleData);

        Suave.Bid memory bid = Suave.newBid(decryptionCondition, bidAllowedPeekers, bidAllowedStores, "default:v0:ethBundles");

        Suave.confidentialStore(bid.id, "default:v0:ethBundles", bundleData);
        Suave.confidentialStore(bid.id, "default:v0:ethBundleSimResults", abi.encode(egp));

        return emitAndReturn(bid, bundleData);
    }

    function emitAndReturn(Suave.Bid memory bid, bytes memory) internal virtual returns (bytes memory) {
        emit BidEvent(bid.id, bid.decryptionCondition, bid.allowedPeekers);
        return bytes.concat(this.emitBid.selector, abi.encode(bid));
    }

    function fetchBidConfidentialBundleData() public returns (bytes memory) {
        require(Suave.isConfidential());

        bytes memory confidentialInputs = Suave.confidentialInputs();
        return abi.decode(confidentialInputs, (bytes));
    }

    // Bids to this contract should not be trusted!
    function emitBid(Suave.Bid calldata bid) public {
        emit BidEvent(bid.id, bid.decryptionCondition, bid.allowedPeekers);
    }
}

