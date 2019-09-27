pragma solidity ^0.5.0;

contract TimeTest {
    function getTime() public view returns (uint256) {
        return now;
    }
}
