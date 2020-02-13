pragma solidity ^0.5.0;

contract TimeTest {
    uint256 storedTime;

    function storeCurrentTime() public {
        storedTime = now;
    }

    function getStoredTime() public view returns (uint256) {
        return storedTime;
    }

    function getCurrentTime() public view returns (uint256) {
        return now;
    }
}
