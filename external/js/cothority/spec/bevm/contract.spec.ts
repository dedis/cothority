// For debugging
// import Log from "../../src/log";

import { EvmAccount, EvmContract } from "../../src/bevm";

describe("EvmContract", () => {
    const candyBytecode = Buffer.from(`
608060405234801561001057600080fd5b506040516020806101cb833981018060405281019080\
805190602001909291905050508060008190555080600181905550600060028190555050610172\
806100596000396000f30060806040526004361061004c576000357c0100000000000000000000\
000000000000000000000000000000000000900463ffffffff168063a1ff2f5214610051578063\
ea319f281461007e575b600080fd5b34801561005d57600080fd5b5061007c6004803603810190\
80803590602001909291905050506100a9565b005b34801561008a57600080fd5b506100936101\
3c565b6040518082815260200191505060405180910390f35b6001548111151515610123576040\
517f08c379a0000000000000000000000000000000000000000000000000000000008152600401\
8080602001828103825260058152602001807f6572726f72000000000000000000000000000000\
00000000000000000000000081525060200191505060405180910390fd5b806001540360018190\
5550806002540160028190555050565b60006001549050905600a165627a7a723058207721a45f\
17c0e0f57e255f33575281d17f1a90d3d58b51688230d93c460a19aa0029
`.trim(), "hex");
    const candyAbi = `
[{"constant":false,"inputs":[{"name":"candies","type":"uint256"}],"name":"eatC\
andy","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"func\
tion"},{"constant":true,"inputs":[],"name":"getRemainingCandies","outputs":[{"\
name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"f\
unction"},{"inputs":[{"name":"_candies","type":"uint256"}],"payable":false,"st\
ateMutability":"nonpayable","type":"constructor"}]
`.trim();

    it("should correctly compute addresses", () => {
        const privKey = Buffer.from(
            "c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3",
            "hex");
        const expectedContractAddress = Buffer.from(
            "8cdaf0cd259887258bc13a92c0a6da92698644c0", "hex");

        const account = new EvmAccount("test", privKey);
        const contract = new EvmContract("candy", candyBytecode, candyAbi);
        // Constructor, 1 transaction, 1 view method
        expect(contract.methodAbi.size).toBe(3);

        contract.createNewAddress(account);
        expect(contract.addresses.length).toBe(1);
        expect(contract.addresses[0]).toEqual(expectedContractAddress);

        account.incNonce();
        contract.createNewAddress(account);
        expect(contract.addresses.length).toBe(2);
        expect(contract.addresses[0]).toEqual(expectedContractAddress);
        expect(contract.addresses[1]).not.toEqual(expectedContractAddress);
    });

    it("should be able to serialize and deserialize", () => {
        const contract = new EvmContract("candy", candyBytecode, candyAbi);

        const ser = contract.serialize();
        const contract2 = EvmContract.deserialize(ser);

        expect(contract.name).toEqual(contract2.name);
        expect(contract.bytecode).toEqual(contract2.bytecode);
        expect(contract.abiJson).toEqual(contract2.abiJson);
        expect(contract.methodAbi).toEqual(contract2.methodAbi);
        expect(contract.addresses).toEqual(contract2.addresses);
    });
});
