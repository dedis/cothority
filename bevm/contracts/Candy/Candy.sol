pragma solidity ^0.4.24;


contract Candy {
    // Fields
    uint256 initialCandies;
    uint256 remainingCandies;
    uint256 eatenCandies;

    // Enumerations

    // Constructor
    constructor (uint256 _candies) public {
        initialCandies = _candies;
        remainingCandies = _candies;
        eatenCandies = 0;
    }

    // Public functions
    function getRemainingCandies () public view returns (uint256) {
        return remainingCandies;
    }

    function eatCandy (uint256 candies) public {
        require(candies <= remainingCandies, "error");
        remainingCandies = remainingCandies - candies;
        eatenCandies = eatenCandies + candies;

    }

    // Private functions
    function invariant () view private returns (bool) {
        return eatenCandies <= initialCandies && remainingCandies <= initialCandies && initialCandies - eatenCandies == remainingCandies;
    }


}

