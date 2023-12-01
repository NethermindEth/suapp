pragma solidity 0.8.19;

import "forge-std/Test.sol";
import "../mempool.sol";

contract MempoolContractTest is Test {

    function test_that_always_works() public {
        assertTrue(true);
    }

    function test_happy_path_through_the_mempool() public {
        Mempool mempool = new Mempool();
        address user1 = address(0x1);
        address user2 = address(0x2);

        emit log("I can log from tests too!");
        vm.prank(user1);
        mempool.submitTx(0);

        vm.prank(user2);
        mempool.submitTx(1);

        Tx[] memory txs = mempool.getTxs();

        assertEq(txs.length, 2);

        assertEq(txs[0].from, user1);
        assertEq(txs[1].from, user2);
        assertEq(txs[1].value, 1);
    }
}
