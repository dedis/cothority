import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import DarcInstance from "../../src/byzcoin/contracts/darc-instance";
import { BLOCK_INTERVAL, ROSTER, SIGNER, startConodes } from "../support/conondes";
import {IdentityWrapper, IdentityTsm, Darc, IdentityDarc, Rule, SignerEd25519} from "../../src/darc";
import {randomBytes} from "crypto";

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

    it("should find tsm authorization", async() => {
        const darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        rpc.setParallel(1);
        const di = await DarcInstance.fromByzcoin(rpc, darc.getBaseID());

        const id = new IdentityTsm({publickey: randomBytes(32)});
        const newDarc = darc.evolve();
        newDarc.addIdentity("spawn:test", id, Rule.OR);
        await di.evolveDarcAndWait(newDarc, [SIGNER], 10);

        const rules = await rpc.checkAuthorization(rpc.genesisID, darc.getBaseID(), IdentityWrapper.fromIdentity(id));
        expect(rules).toEqual(["spawn:test"]);
    })
});
