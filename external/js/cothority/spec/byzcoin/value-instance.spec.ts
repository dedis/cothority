import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import ValueInstance from "../../src/byzcoin/contracts/value-instance";
import { Rule } from "../../src/darc/rules";
import { BLOCK_INTERVAL, ROSTER, SIGNER, startConodes } from "../support/conondes";

describe("ValueInstance Tests", () => {
    const roster = ROSTER.slice(0, 4);

    beforeAll(async () => {
        await startConodes();
    });

    it("should spawn a value instance", async () => {
        const darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);
        darc.addIdentity("spawn:value", SIGNER, Rule.OR);
        darc.addIdentity("invoke:value.update", SIGNER, Rule.OR);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const value = Buffer.from("value instance");
        const vi = await ValueInstance.spawn(rpc, darc.getBaseID(), [SIGNER], value);

        expect(vi.value).toEqual(value);

        const vi2 = await ValueInstance.fromByzcoin(rpc, vi.id);
        expect(vi2.value).toEqual(value);
        const value2 = Buffer.from("new value of value instance");
        await vi2.updateValue([SIGNER], value2);

        expect(vi.value).toEqual(value);
        await vi.update();
        expect(vi.value).toEqual(value2);
    });
});
