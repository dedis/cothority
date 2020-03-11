pragma solidity ^0.4.24;
pragma experimental ABIEncoderV2;

contract CallByzcoin {
    // Data structures

    struct Argument {
        string name;
        bytes value;
    }

    // Events

    event ByzcoinSpawn (
        bytes32 instanceID,
        string contractID,
        Argument[] args
    );

    event ByzcoinInvoke (
        bytes32 instanceID,
        string contractID,
        string command,
        Argument[] args
    );

    event ByzcoinDelete (
        bytes32 instanceID,
        string contractID,
        Argument[] args
    );

    // Fields

    // Constructor

    constructor() public {
    }

    // Public functions

    function spawnValue(bytes32 darcID, string contractID, uint8 value) public {
        bytes memory argValue = new bytes(1);
        argValue[0] = byte(value);

        Argument[] memory args = new Argument[](1);
        args[0] = Argument({
            name: "value",
            value: argValue
        });

        emit ByzcoinSpawn(darcID, contractID, args);
    }

    function spawnTwoValues(bytes32 darcID, string contractID, uint8 value) public {
        bytes memory argValue = new bytes(1);
        argValue[0] = byte(value);

        Argument[] memory args = new Argument[](1);
        args[0] = Argument({
            name: "value",
            value: argValue
        });

        emit ByzcoinSpawn(darcID, contractID, args);
        emit ByzcoinSpawn(darcID, contractID, args);
    }

    function updateValue(bytes32 instanceID, string contractID, uint8 value) public {
        bytes memory argValue = new bytes(1);
        argValue[0] = byte(value);

        Argument[] memory args = new Argument[](1);
        args[0] = Argument({
            name: "value",
            value: argValue
        });

        emit ByzcoinInvoke(instanceID, contractID, "update", args);
    }

    function deleteValue(bytes32 instanceID, string contractID) public {
        Argument[] memory args = new Argument[](0);

        emit ByzcoinDelete(instanceID, contractID, args);
    }

    // Private functions
}

