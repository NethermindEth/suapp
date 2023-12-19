pragma solidity ^0.8.8;

import "../../suave-geth/suave/sol/libraries/Suave.sol";

// Builder contract
contract OpBuilder {
    event BidEvent(
        Suave.BidId bidId,
        uint64 decryptionCondition,
        address[] allowedPeekers
    );

    event BuilderBidEvent(
        Suave.BidId bidId,
        bytes builderBid
    );

    // Emitter helpers
    function emitBuilderBidAndBid(Suave.Bid memory bid, bytes memory builderBid) public returns (Suave.Bid memory, bytes memory) {
        emit BuilderBidEvent(bid.id, builderBid);
        emit BidEvent(bid.id, bid.decryptionCondition, bid.allowedPeekers);
        return (bid, builderBid);
    }

    function emitBid(Suave.Bid calldata bid) public {
        emit BidEvent(bid.id, bid.decryptionCondition, bid.allowedPeekers);
    }

    // INFO: Since for now we don't implement the IBundle, we do a shortcut and receive the Txs (or Bundles - to be decided) directly here.
    function newTx(uint64 blockHeight, address[] memory bidAllowedPeekers, address[] memory bidAllowedStores) external payable returns (bytes memory) {
        require(Suave.isConfidential());

        bytes memory bundleData = fetchBidConfidentialBundleData();

        Suave.Bid memory bid = Suave.newBid(blockHeight, bidAllowedPeekers, bidAllowedStores, "default:v0:ethBundles");

        Suave.confidentialStore(bid.id, "default:v0:ethBundles", bundleData);

        emit BidEvent(bid.id, bid.decryptionCondition, bid.allowedPeekers);
        return bytes.concat(this.emitBid.selector, abi.encode(bid));
    }

    function buildBlock(Suave.BuildBlockArgs memory blockArgs, uint64 blockHeight) public returns (bytes memory) {
        require(Suave.isConfidential());

        Suave.Bid[] memory allBids = Suave.fetchBids(blockHeight, "default:v0:ethBundles");
        if (allBids.length == 0) {
            // TODO: we should build empty blocks as well
            revert Suave.PeekerReverted(address(this), "no bids");
        }

        Suave.BidId[] memory allBidIds = new Suave.BidId[](allBids.length);
        for (uint i = 0; i < allBids.length; i++) {
            allBidIds[i] = allBids[i].id;
        }

        (Suave.Bid memory blockBid, bytes memory builderBid) = this.doBuild(blockArgs, blockHeight, allBidIds, "");
        emit BuilderBidEvent(blockBid.id, builderBid);
        emit BidEvent(blockBid.id, blockBid.decryptionCondition, blockBid.allowedPeekers);
        return bytes.concat(this.emitBuilderBidAndBid.selector, abi.encode(blockBid, builderBid));
    }

    function doBuild(Suave.BuildBlockArgs memory blockArgs,
        uint64 blockHeight,
        Suave.BidId[] memory bids,
        string memory namespace
    ) public view returns (Suave.Bid memory, bytes memory) {

        address[] memory allowedPeekers = new address[](3);
        allowedPeekers[0] = address(this);
        allowedPeekers[1] = Suave.BUILD_ETH_BLOCK;

        Suave.Bid memory blockBid = Suave.newBid(blockHeight, allowedPeekers, allowedPeekers, "default:v0:mergedBids");
        Suave.confidentialStore(blockBid.id, "default:v0:mergedBids", abi.encode(bids));

        (bytes memory builderBid, bytes memory payload) = Suave.buildEthBlock(blockArgs, blockBid.id, namespace);
        Suave.confidentialStore(blockBid.id, "default:v0:builderPayload", payload); // only through this.unlock

        return (blockBid, builderBid);
    }

    // INFO: This function is called by MevBoost to fetch the block payload and expose it to the sequencer.
    function getBlock(Suave.BidId bidId) public view returns (bytes memory) {
        require(Suave.isConfidential());

        // TODO: Access control?
        bytes memory payload = Suave.confidentialRetrieve(bidId, "default:v0:builderPayload");
        return payload;
    }

    function fetchBidConfidentialBundleData() public view returns (bytes memory) {
        require(Suave.isConfidential());

        bytes memory confidentialInputs = Suave.confidentialInputs();
        return abi.decode(confidentialInputs, (bytes));
    }
}
