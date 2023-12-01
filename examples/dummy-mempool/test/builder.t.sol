pragma solidity 0.8.19;

import "forge-std/Test.sol";
import "../mempool.sol";
import "../builder.sol";

contract BuilderContractTest is Test {
    function test_happy_path_through_the_builder() public {
        Mempool mempool = new Mempool();
        Builder builder = new Builder(mempool);

        mempool.submitTx(1);
        mempool.submitTx(10);

        assertEq(builder.getBlockNum(), 0);
        assertEq(builder.getBlockValue(), 0);
        builder.build();

        assertEq(builder.getBlockNum(), 1);

        mempool.submitTx(100);
        assertEq(builder.getBlockValue(), 11);

        builder.build();
        assertEq(builder.getBlockNum(), 2);
        assertEq(builder.getBlockValue(), 100);
    }

    function test_reverts_when_no_transactions() public {
        Mempool mempool = new Mempool();
        Builder builder = new Builder(mempool);

        vm.expectRevert("No transactions to build block with");
        builder.build();
    }

    function test_reverts_when_no_value0() public {
        Mempool mempool = new Mempool();
        Builder builder = new Builder(mempool);

        mempool.submitTx(0);
        mempool.submitTx(0);
        mempool.submitTx(0);

        vm.expectRevert("No transactions to build block with");
        builder.build();
    }
}