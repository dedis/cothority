import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import DarcInstance from "../../src/byzcoin/contracts/darc-instance";
import Darc from "../../src/darc/darc";
import IdentityDarc from "../../src/darc/identity-darc";
import { Rule } from "../../src/darc/rules";
import SignerEd25519 from "../../src/darc/signer-ed25519";
import { BLOCK_INTERVAL, ROSTER, SIGNER, startConodes } from "../support/conondes";

describe("DarcInstance Tests", () => {
    const roster = ROSTER.slice(0, 4);

    beforeAll(async () => {
        await startConodes();
    });

    it("should find related rule", async () => {
        const darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);
        darc.addIdentity("spawn:darc", SIGNER, Rule.OR);
        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);

        const sig = SignerEd25519.random();
        const d3 = Darc.createBasic([SIGNER], [sig], Buffer.from("sub-darc"));
        const d2 = Darc.createBasic([SIGNER], [new IdentityDarc({id: d3.getBaseID()})]);
        const d1 = Darc.createBasic([SIGNER], [new IdentityDarc({id: d2.getBaseID()})]);
        const di1 = await DarcInstance.spawn(rpc, darc.getBaseID(), [SIGNER], d1);
        const di2 = await DarcInstance.spawn(rpc, darc.getBaseID(), [SIGNER], d2);
        const di3 = await DarcInstance.spawn(rpc, darc.getBaseID(), [SIGNER], d3);
        expect(di1.ruleMatch(Darc.ruleSign, [sig])).toBeTruthy();
        expect(di1.ruleMatch(Darc.ruleSign, [new IdentityDarc({id: d2.getBaseID()})])).toBeTruthy();
    });
});
