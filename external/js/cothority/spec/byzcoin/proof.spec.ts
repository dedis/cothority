import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import DarcInstance from "../../src/byzcoin/contracts/darc-instance";
import Proof from "../../src/byzcoin/proof";
import Darc from "../../src/darc/darc";
import SignerEd25519 from "../../src/darc/signer-ed25519";
import { BLOCK_INTERVAL, ROSTER, SIGNER, startConodes } from "../support/conondes";

describe("Proof Tests", () => {
    const roster = ROSTER.slice(0, 4);

    let darc: Darc;
    let rpc: ByzCoinRPC;
    let di: DarcInstance;

    beforeAll(async () => {
        await startConodes();

        darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);
        rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        di = await DarcInstance.fromByzcoin(rpc, darc.baseID);
    });

    it("should get proofs and verify them", async () => {
        const n = 4;
        const ids: Buffer[] = [];

        for (let i = 0; i < n; i++) {
            const newDarc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster, `Darc n°${i}`);
            await di.spawnDarcAndWait(newDarc, [SIGNER], 10);
            ids.push(newDarc.baseID);
        }

        for (const id of ids) {
            const p = await rpc.getProof(id);
            expect(p.exists(id)).toBeTruthy();
            expect(p.toString()).toBeDefined();
            expect(p.matchContract(id));
        }
    });

    it("should refuse a proof for a non-existing key", async () => {
        const badKey = Buffer.from("123");
        const badProof = await rpc.getProof(badKey);
        expect(badProof.exists(Buffer.from(badKey))).toBeFalsy();
    });

    it("should throw for corrupted proofs", async () => {
        let p = await rpc.getProof(darc.baseID);
        p.inclusionproof.interiors[p.inclusionproof.interiors.length - 1].right.writeInt32LE(1, 0);
        expect(() => p.exists(darc.baseID)).toThrowError("invalid interior node");

        p = await rpc.getProof(darc.baseID);
        p.inclusionproof.leaf.key.writeInt32LE(1, 0);
        expect(() => p.exists(darc.baseID))
            .toThrowError("no corresponding leaf/empty node with respect to the interior node");
    });
});

describe("Proof Offline Tests", () => {
    it("should throw for invalid proofs", () => {
        let key = Buffer.from([]);

        const p1 = Proof.fromObject({ inclusionproof: { interiors: [] } });
        expect(() => p1.exists(key)).toThrowError("key is nil");

        key = Buffer.from("123");
        expect(() => p1.exists(key)).toThrowError("no interior node");
    });
});
