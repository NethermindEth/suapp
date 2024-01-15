// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.8;

import "../../suave-geth/suave/sol/libraries/Suave.sol";

// Builder contract
contract OpBuilder {
    address[] public addressList = [Suave.ANYALLOWED, Suave.BUILD_ETH_BLOCK];

    event NewBundleEvent(
        Suave.DataId dataId,
        uint64 decryptionCondition,
        address[] allowedPeekers
    );

    event NewBuilderBidEvent(
        Suave.DataId dataId,
        uint64 decryptionCondition,
        address[] allowedPeekers,
        bytes envelope
    );

    // Emitter helpers
    function emitNewBuilderBidEvent(Suave.DataRecord memory record, bytes memory envelope) public {
        emit NewBuilderBidEvent(record.id, record.decryptionCondition, record.allowedPeekers, envelope);
    }

    function emitNewBundleEvent(Suave.DataRecord calldata record) public {
        emit NewBundleEvent(record.id, record.decryptionCondition, record.allowedPeekers);
    }

    // INFO: Since for now we don't implement the IBundle, we do a shortcut and receive the Txs (or Bundles - to be decided) directly here.
    function newTx(uint64 blockNumber) external payable returns (bytes memory) {
        require(Suave.isConfidential());
        bytes memory bundleData = Suave.confidentialInputs();

        // Allow anyone to peek at the bundle
        Suave.DataRecord memory record = Suave.newDataRecord(blockNumber, addressList, addressList, "default:v0:ethBundles");
        Suave.confidentialStore(record.id, "default:v0:ethBundles", bundleData);

        emit NewBundleEvent(record.id, record.decryptionCondition, record.allowedPeekers);
        return bytes.concat(this.emitNewBundleEvent.selector, abi.encode(record));
    }

    function buildBlock(
        uint64 blockNumber
    ) public returns (bytes memory) {
        require(Suave.isConfidential());
        Suave.DataRecord[] memory allRecords = Suave.fetchDataRecords(blockNumber, "default:v0:ethBundles");
        if (allRecords.length == 0) {
            // TODO: we should build empty blocks as well
            revert Suave.PeekerReverted(address(this), "no bundles in default:v0:ethBundles");
        }

        Suave.DataId[] memory allRecordIds = new Suave.DataId[](allRecords.length);
        for (uint i = 0; i < allRecords.length; i++) {
            allRecordIds[i] = allRecords[i].id;
        }

        // NOTE: these are all arbitrary values for now. Attributes are handled by
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

        (Suave.DataRecord memory mergedDataRecord, Suave.DataRecord memory builderBidRecord, bytes memory envelope) = doBuild(
            blockArgs, allRecordIds, "default:v0:ethBundles", blockNumber);
        return bytes.concat(this.emitNewBuilderBidEvent.selector, abi.encode(builderBidRecord, envelope));
    }

    function doBuild(
        Suave.BuildBlockArgs memory blockArgs,
        Suave.DataId[] memory recordIds,
        string memory namespace,
        uint64 blockNumber
    ) internal view returns (Suave.DataRecord memory, Suave.DataRecord memory, bytes memory) {
        Suave.DataRecord memory mergedDataRecord = Suave.newDataRecord(blockNumber, addressList, addressList, "default:v0:mergedDataRecords");
        Suave.confidentialStore(mergedDataRecord.id, "default:v0:mergedDataRecords", abi.encode(recordIds));

        (bytes memory builderBid, bytes memory envelope) = Suave.buildEthBlock(blockArgs, mergedDataRecord.id, namespace); // namespace is not used.
        Suave.DataRecord memory builderBidRecord = Suave.newDataRecord(blockNumber, addressList, addressList, "default:v0:builderBids");
        Suave.confidentialStore(builderBidRecord.id, "default:v0:builderBids", builderBid);
        return (mergedDataRecord, builderBidRecord, envelope);
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
