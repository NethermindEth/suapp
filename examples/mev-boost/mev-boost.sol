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
//        uint n = bidsByEGP.length;
//        for (uint i = 0; i < n - 1; i++) {
//            for (uint j = i + 1; j < n; j++) {
//                if (bidsByEGP[i].egp < bidsByEGP[j].egp) {
//                    EgpBidPair memory temp = bidsByEGP[i];
//                    bidsByEGP[i] = bidsByEGP[j];
//                    bidsByEGP[j] = temp;
//                }
//            }
//        }
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

contract OFAPrivate {
    address[] public addressList = [0xC8df3686b4Afb2BB53e60EAe97EF043FE03Fb829];

    // Struct to hold hint-related information for an order.
    struct HintOrder {
        Suave.BidId id;
        bytes hint;
    }

    event HintEvent (
        Suave.BidId id,
        bytes hint
    );

    // Internal function to save order details and generate a hint.
    function saveOrder() internal view returns (HintOrder memory) {
        // Retrieve the bundle data from the confidential inputs
        bytes memory bundleData = Suave.confidentialInputs();

        // Simulate the bundle and extract its score.
        uint64 egp = Suave.simulateBundle(bundleData);

        // Extract a hint about this bundle that is going to be leaked
        // to external applications.
        bytes memory hint = Suave.extractHint(bundleData);

        // Store the bundle and the simulation results in the confidential datastore.
        Suave.Bid memory bid = Suave.newBid(10, addressList, addressList, "");
        Suave.confidentialStore(bid.id, "default:v0:ethBundles", bundleData);
        Suave.confidentialStore(bid.id, "default:v0:ethBundleSimResults", abi.encode(egp));

        HintOrder memory hintOrder;
        hintOrder.id = bid.id;
        hintOrder.hint = hint;

        return hintOrder;
    }

    function emitHint(HintOrder memory order) public payable {
        emit HintEvent(order.id, order.hint);
    }

    // Function to create a new user order
    function newOrder() external payable returns (bytes memory) {
        HintOrder memory hintOrder = saveOrder();
        return abi.encodeWithSelector(this.emitHint.selector, hintOrder);
    }

    // Function to match and backrun another bid.
    function newMatch(Suave.BidId shareBidId) external payable returns (bytes memory) {
        HintOrder memory hintOrder = saveOrder();

        // Merge the bids and store them in the confidential datastore.
        // The 'fillMevShareBundle' precompile will use this information to send the bundles.
        Suave.BidId[] memory bids = new Suave.BidId[](2);
        bids[0] = shareBidId;
        bids[1] = hintOrder.id;
        Suave.confidentialStore(hintOrder.id, "default:v0:mergedBids", abi.encode(bids));

        return abi.encodeWithSelector(this.emitHint.selector, hintOrder);
    }
}
