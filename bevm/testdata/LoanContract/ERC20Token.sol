pragma solidity ^0.4.24;


interface ERC20Token {

    function balanceOf(address from) external returns (uint256);
    function transfer(address to, uint256 amount) external returns (bool);
}

