pragma solidity ^0.8.8;

struct Tx {
    address from;
    uint8 value;
}

contract Mempool {
    Tx[] public txs;

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
}
