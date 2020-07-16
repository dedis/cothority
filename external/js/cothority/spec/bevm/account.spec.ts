// For debugging
// import Log from "../../src/log";

import { EvmAccount } from "../../src/bevm";

describe("EvmAccount", async () => {
    it("should correctly compute its address", () => {
        const privKey = Buffer.from("c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3", "hex");
        const expectedAccountAddress = Buffer.from("627306090abab3a6e1400e9345bc60c78a8bef57", "hex");

        const account = new EvmAccount("test", privKey);

        expect(account.address).toEqual(expectedAccountAddress);
    });

    it("should be able to serialize and deserialize", () => {
        const privKey = Buffer.from("c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3", "hex");
        const account = new EvmAccount("test", privKey);
        account.incNonce();

        const ser = account.serialize();
        const account2 = EvmAccount.deserialize(ser);

        const data = Buffer.from("this is a test");

        expect(account.address).toEqual(account2.address);
        expect(account.nonce).toEqual(account2.nonce);
        expect(account.sign(data)).toEqual(account2.sign(data));
    });
});
