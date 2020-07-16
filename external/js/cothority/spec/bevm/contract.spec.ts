// For debugging
// import Log from "../../src/log";

import { EvmAccount, EvmContract } from "../../src/bevm";

describe("EvmContract", () => {
    /* tslint:disable:max-line-length */
    const candyBytecode = Buffer.from(`
608060405234801561001057600080fd5b506040516020806101cb833981018060405281019080805190602001909291905050508060008190555080600181905550600060028190555050610172806100596000396000f30060806040526004361061004c576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063a1ff2f5214610051578063ea319f281461007e575b600080fd5b34801561005d57600080fd5b5061007c600480360381019080803590602001909291905050506100a9565b005b34801561008a57600080fd5b5061009361013c565b6040518082815260200191505060405180910390f35b6001548111151515610123576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260058152602001807f6572726f7200000000000000000000000000000000000000000000000000000081525060200191505060405180910390fd5b8060015403600181905550806002540160028190555050565b60006001549050905600a165627a7a723058207721a45f17c0e0f57e255f33575281d17f1a90d3d58b51688230d93c460a19aa0029
`.trim(), "hex");
    const candyAbi = `
[{"constant":false,"inputs":[{"name":"candies","type":"uint256"}],"name":"eatCandy","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"getRemainingCandies","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"inputs":[{"name":"_candies","type":"uint256"}],"payable":false,"stateMutability":"nonpayable","type":"constructor"}]
`.trim();
    /* tslint:enable:max-line-length */

    it("should correctly compute addresses", () => {
        const privKey = Buffer.from("c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3", "hex");
        const expectedContractAddress = Buffer.from("8cdaf0cd259887258bc13a92c0a6da92698644c0", "hex");

        const account = new EvmAccount("test", privKey);
        const contract = new EvmContract("candy", candyBytecode, candyAbi);

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
        expect(contract.abi).toEqual(contract2.abi);
        expect(contract.addresses).toEqual(contract2.addresses);
    });
});
