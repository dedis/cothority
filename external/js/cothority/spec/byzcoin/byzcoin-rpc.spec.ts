import { ByzCoinRPC } from "../../src/byzcoin";
import DarcInstance from "../../src/byzcoin/contracts/darc-instance";
import Instance from "../../src/byzcoin/instance";
import { BLOCK_INTERVAL, ROSTER, SIGNER, startConodes } from "../support/conondes";

describe("ByzCoinRPC Tests", () => {
    const roster = ROSTER.slice(0, 4);

    beforeAll(async () => {
        await startConodes();
    });

    it("should create an rpc and evolve/spawn darcs", async () => {
        expect(() => ByzCoinRPC.makeGenesisDarc([], roster)).toThrow();

        const darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);
        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);

        const proof = await rpc.getProof(Buffer.alloc(32, 0));
        expect(proof).toBeDefined();

        const instance = await DarcInstance.fromByzcoin(rpc, darc.getGenesisDarcID());

        const evolveDarc = darc.evolve();
        const evolveInstance = await instance.evolveDarcAndWait(evolveDarc, [SIGNER], 10);
        expect(evolveInstance.getDarc().getGenesisDarcID()).toEqual(darc.getGenesisDarcID());

        await evolveInstance.update();

        const newDarc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster, "another darc");
        const newInstance = await instance.spawnDarcAndWait(newDarc, [SIGNER], 10);
        expect(newInstance.getDarc().getGenesisDarcID().equals(newDarc.getGenesisDarcID())).toBeTruthy();
    });

    it("should create an rpc and get it from byzcoin", async () => {
        const darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);
        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);

        const rpc2 = await ByzCoinRPC.fromByzcoin(roster, rpc.getGenesis().hash);
        await rpc2.updateConfig();

        expect(rpc.getDarc().id).toEqual(rpc2.getDarc().id);
        expect(rpc2.getConfig().blockInterval.toNumber()).toEqual(rpc.getConfig().blockInterval.toNumber());
    });

    it("should throw an error for non-existing instance or wrong type", async () => {
        const darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);
        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);

        await expectAsync(Instance.fromByzCoin(rpc, Buffer.from([1, 2, 3])))
            .toBeRejectedWith(new Error("key not in proof: 010203"));
        await expectAsync(DarcInstance.fromByzcoin(rpc, Buffer.alloc(32, 0))).toBeRejected();
    });
});
