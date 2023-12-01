pragma solidity ^0.8.8;

import {Tx, Mempool} from "./mempool.sol";

//error NoValueToBuildBlockWith();
contract Builder {
    struct Block {
        uint256 blockNumber;
        uint16 value;
    }

    Block[] public blocks;
    Mempool public mempool;

    constructor(Mempool _mempool) {
        mempool = _mempool;

        Block memory genesis = Block(0, 0);
        blocks.push(genesis);
    }

    function build() public {
        uint16 sumOfValues = 0;
        Tx[] memory transactions = mempool.popTxs();
        for (uint256 i = 0; i < transactions.length; i++) {
            sumOfValues += transactions[i].value;
        }

        require(sumOfValues > 0, "No transactions to build block with");
        Block memory blk = Block(blocks.length, sumOfValues);
        blocks.push(blk);
    }

    function getBlockNum() public view returns (uint256) {
        return blocks.length-1;
    }

    function getBlockValue() public view returns (uint16) {
        return blocks[blocks.length-1].value;
    }
}