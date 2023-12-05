pragma solidity ^0.8.8;

import "../../suave-geth/suave/sol/libraries/Suave.sol";

struct Tx {
    address from;
    uint8 value;
}



contract Mempool {
    Tx[] public txs;
    address[] public addressList = [0xC8df3686b4Afb2BB53e60EAe97EF043FE03Fb829];

    struct HintOrder {
        Suave.BidId id;
        uint8 hint;
    }

    event HintEvent (
        Suave.BidId id,
        uint8 hint
    );

    function getTxs() public view returns (Tx[] memory) {
        return txs;
    }

    function popTxs() public returns (Tx[] memory) {
        Tx[] memory _txs = txs;
        delete txs;
        return _txs;
    }

    function submitTx(uint8 value) public {
        Tx memory trx = Tx(msg.sender, value);
        txs.push(trx);
    }

    function submitConfidentialTx() public returns (bytes memory){
        bytes memory confidentialInputs = Suave.confidentialInputs();
        // how to decode bytes abi / rlp?
        uint8 secretValue = abi.decode(confidentialInputs, (uint8));
        submitTx(secretValue);

        // Store the bundle and the simulation results in the confidential datastore.
        Suave.Bid memory bid = Suave.newBid(10, addressList, addressList, "");
        Suave.confidentialStore(bid.id, "mempool:v0:Txs", confidentialInputs);

        HintOrder memory hintOrder = HintOrder(bid.id, secretValue);
        return abi.encodeWithSelector(this.emitHint.selector, hintOrder);
    }

    function emitHint(HintOrder memory order) public payable {
        emit HintEvent(order.id, order.hint);
    }


}
