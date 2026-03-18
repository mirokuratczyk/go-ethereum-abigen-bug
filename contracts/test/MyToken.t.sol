// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../src/MyToken.sol";

contract MyTokenTest is Test {
    MyToken public token;

    function setUp() public {
        token = new MyToken();
    }

    function test_NameAndSymbol() public view {
        assertEq(token.name(), "MyToken");
        assertEq(token.symbol(), "MTK");
    }

    function test_InitialSupplyIsZero() public view {
        assertEq(token.totalSupply(), 0);
    }
}
