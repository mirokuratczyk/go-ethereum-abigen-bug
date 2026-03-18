// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/// @title MyToken
/// @notice A simple ERC20 token.
contract MyToken is ERC20 {
    /// @dev Deploys the contract and sets the name and symbol for the token.
    constructor() ERC20("MyToken", "MTK") {}

    /// @notice Mint tokens to an address.
    /// @param to Recipient address.
    /// @param amount Amount of tokens to mint (in wei units).
    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }
}
