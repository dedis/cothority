pragma solidity ^0.4.24;
pragma experimental ABIEncoderV2;

contract Verify {
    // Data structures

    struct Argument {
        string name;
        bytes value;
    }

    // Constructor

    constructor () public {
    }

    // Public functions

    // Validation function that checks whether the "value" argument is larger
    // than the current state (passed in "extra").
    // To simplify, just compare one byte.
    function isGreater(
        bytes32 instanceID,
        string action,
        Argument[] arguments,
        int64 protocolVersion,
        int64 skipBlockIndex,
        bytes extra
    ) public view returns (string error) {
        uint8 currentState = uint8(extra[0]);
        uint8 value = 0;

        // Find value in "value" argument
        for (uint i = 0; i < arguments.length; i++) {
            // Cannot compare strings directly
            if (keccak256(abi.encodePacked(arguments[i].name)) ==
                keccak256(abi.encodePacked("value"))) {
                value = uint8(arguments[i].value[0]);
                break;
            }
        }

        if (value > currentState) {
            return "";
        }

        return "value is not greater than current state";
    }

    // Private functions
}

