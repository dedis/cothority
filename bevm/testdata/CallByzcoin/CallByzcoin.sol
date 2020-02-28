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

    uint8 counter;

    // Constructor

    constructor() public {
        counter = 1;
    }

    // Public functions

    function spawnValue(bytes32 darcID, string contractID, uint8 value) public {
        bytes memory argValue = new bytes(1);
        argValue[0] = byte(value);

        bytes memory id = new bytes(4);
        id[0] = byte("v");
        id[1] = byte("a");
        id[2] = byte("l");
        id[3] = byte(counter);

        Argument[] memory args = new Argument[](2);
        args[0] = Argument({
            name: "value",
            value: argValue
        });
        args[1] = Argument({
            name: "id",
            value: id
        });

        counter++;

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

