// For debugging
// import Log from "../../src/log";

import { EvmAccount } from "../../src/bevm";

import { Transaction } from "ethereumjs-tx";

describe("EvmAccount", async () => {
    it("should correctly compute its address", () => {
        const privKey = Buffer.from(
            "c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3",
            "hex");
        const expectedAccountAddress = Buffer.from(
            "627306090abab3a6e1400e9345bc60c78a8bef57", "hex");

        const account = new EvmAccount("test", privKey);

        expect(account.address).toEqual(expectedAccountAddress);
    });

    it("should be able to serialize and deserialize", () => {
        const privKey = Buffer.from(
            "c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3",
            "hex");
        const account = new EvmAccount("test", privKey);
        account.incNonce();

        const ser = account.serialize();
        const account2 = EvmAccount.deserialize(ser);

        const tx = new Transaction({
            data: Buffer.from("0102030405060708090a0b0c0d0e0f00", "hex"),
        });

        expect(account.address).toEqual(account2.address);
        expect(account.nonce).toEqual(account2.nonce);
        expect(account.sign(tx)).toEqual(account2.sign(tx));
    });
});
