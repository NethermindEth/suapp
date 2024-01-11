// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.8;

import "../../suave-geth/suave/sol/libraries/Suave.sol";

// Builder contract
contract OpBuilder {
    address[] public addressList = [Suave.ANYALLOWED];

    event BidEvent(
        Suave.DataId bidId,
        uint64 decryptionCondition,
        address[] allowedPeekers
    );

    event BuilderBidEvent(
        Suave.DataId bidId,
        bytes builderBid
    );

    // Emitter helpers
    function emitBuilderBidAndBid(Suave.DataRecord memory record, bytes memory builderBid) public returns (Suave.DataRecord memory, bytes memory) {
        emit BuilderBidEvent(record.id, builderBid);
        emit BidEvent(record.id, record.decryptionCondition, record.allowedPeekers);
        return (record, builderBid);
    }

    function emitBid(Suave.DataRecord calldata record) public {
        emit BidEvent(record.id, record.decryptionCondition, record.allowedPeekers);
    }

    // INFO: Since for now we don't implement the IBundle, we do a shortcut and receive the Txs (or Bundles - to be decided) directly here.
    function newTx() external payable returns (bytes memory) {
        require(Suave.isConfidential());
        bytes memory bundleData = Suave.confidentialInputs();

        // Allow anyone to peek at the bundle
        Suave.DataRecord memory record = Suave.newDataRecord(10, addressList, addressList, "v0");
        Suave.confidentialStore(record.id, "default:v0:ethBundles", bundleData);

        emit BidEvent(record.id, record.decryptionCondition, record.allowedPeekers);
        return bytes.concat(this.emitBid.selector, abi.encode(record));
    }

    function getNumberOfBids() external payable returns (uint) {
        Suave.DataRecord[] memory allRecords = Suave.fetchDataRecords(10, "default:v0:ethBundles");
        return allRecords.length;
    }

    function buildBlock(
    ) public returns (bytes memory) {
        require(Suave.isConfidential());
        Suave.DataRecord[] memory allRecords = Suave.fetchDataRecords(10, "default:v0:ethBundles");
        if (allRecords.length == 0) {
            // TODO: we should build empty blocks as well
            revert Suave.PeekerReverted(address(this), "no bids");
        }

        Suave.DataId[] memory allRecordIds = new Suave.DataId[](allRecords.length);
        for (uint i = 0; i < allRecords.length; i++) {
            allRecordIds[i] = allRecords[i].id;
        }

        // NOTE: these are all arbitrary values for now. The attributes are Handled by
        // the remote rpc.
        Suave.BuildBlockArgs memory blockArgs;

        blockArgs.slot = 0;
        blockArgs.proposerPubkey = "";
        blockArgs.parent = blockhash(block.number);
        blockArgs.timestamp = uint64(0);
        blockArgs.feeRecipient = address(0);
        blockArgs.gasLimit = uint64(0);
        blockArgs.random = blockhash(block.number);

         // Initialize the withdrawals field
        Suave.Withdrawal[] memory tempWithdrawals = new Suave.Withdrawal[](1);

        // Manually setting values for each withdrawal
        tempWithdrawals[0] = Suave.Withdrawal({
            index: 0,
            validator: 0,
            Address: address(0x123), // Example address
            amount: 1000       // Example amount
        });

        blockArgs.withdrawals = tempWithdrawals;
        blockArgs.extra = "";
        blockArgs.fillPending = false;

        (Suave.DataRecord memory blockBid, bytes memory builderBid) = this.doBuild(blockArgs, allRecordIds, "");
        emit BuilderBidEvent(blockBid.id, builderBid);
        emit BidEvent(blockBid.id, blockBid.decryptionCondition, blockBid.allowedPeekers);
        return bytes.concat(this.emitBuilderBidAndBid.selector, abi.encode(blockBid, builderBid));
    }

    function doBuild(Suave.BuildBlockArgs memory blockArgs,
        Suave.DataId[] memory bids,
        string memory namespace
    ) public view returns (Suave.DataRecord memory, bytes memory) {
        Suave.DataRecord memory blockBid = Suave.newDataRecord(10, addressList, addressList, "default:v0:mergedBids");
        Suave.confidentialStore(blockBid.id, "default:v0:mergedBids", abi.encode(bids));

        (bytes memory builderBid, bytes memory payload) = Suave.buildEthBlock(blockArgs, blockBid.id, namespace);
        Suave.confidentialStore(blockBid.id, "default:v0:builderPayload", payload); // only through this.unlock

        return (blockBid, builderBid);
    }

    // INFO: This function is called by MevBoost to fetch the block payload and expose it to the sequencer.
    function getBlock(Suave.DataId bidId) public view returns (bytes memory) {
        require(Suave.isConfidential());

        // TODO: Access control?
        bytes memory payload = Suave.confidentialRetrieve(bidId, "default:v0:builderPayload");
        return payload;
    }

    function emitNothingAfterBlockRetrievedCallback() external payable {
    }

    function postBlockToRelay(string memory relayUrl, bytes memory builderBid) public payable returns (bytes memory) {
        Suave.submitEthBlockToRelay(relayUrl, builderBid);
        return abi.encodeWithSelector(this.emitNothingAfterBlockRetrievedCallback.selector);
    }
}
