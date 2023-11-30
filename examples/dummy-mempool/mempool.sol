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

    function submitTx(address from, uint8 value) public {
        Tx memory trx = Tx(from, value);
        txs.push(trx);
    }
}