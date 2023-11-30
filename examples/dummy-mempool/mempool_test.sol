pragma solidity 0.8.19;

import "forge-std/Test.sol";
import "./mempool.sol";

contract MempoolContractTest is Test {

    function testExample() public {
        assertTrue(true);
    }

    function testMempool() public {
        Mempool mempool = new Mempool();

        mempool.submitTx(address(0x0), 0);
        mempool.submitTx(address(0x1), 1);

        Tx[] memory txs = mempool.getTxs();

        assertEq(txs.length, 2);
        assertEq(txs[0].from, address(0x0));
        assertEq(txs[1].value, 1);
    }
}
