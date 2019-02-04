import { startConodes, ROSTER, BLOCK_INTERVAL } from '../support/conondes';
import ByzCoinRPC from '../../src/byzcoin/byzcoin-rpc';
import SignerEd25519 from '../../src/darc/signer-ed25519';
import DarcInstance from '../../src/byzcoin/contracts/darc-instance';
import Darc from '../../src/darc/darc';
import Proof from '../../src/byzcoin/proof';

describe('Proof Tests', () => {
    const roster = ROSTER.slice(0, 4);
    const admin = SignerEd25519.fromBytes(Buffer.from("0cb119094dbf72dfd169f8ba605069ce66a0c8ba402eb22952b544022d33b90c", "hex"));

    let darc: Darc;
    let rpc: ByzCoinRPC;
    let di: DarcInstance;

    beforeAll(async () => {
        await startConodes();

        darc = ByzCoinRPC.makeGenesisDarc([admin], roster);
        rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        di = await DarcInstance.fromByzcoin(rpc, darc.baseID);
    });

    it('should get proofs en verify them', async () => {
        const n = 4;
        const ids: Buffer[] = [];

        for (let i = 0; i < n; i++) {
            const newDarc = ByzCoinRPC.makeGenesisDarc([admin], roster, `Darc n°${i}`);
            await di.spawnDarcAndWait(newDarc, admin, 10);
            ids.push(newDarc.baseID);
        }

        for (let j = 0; j < ids.length; j++) {
            const p = await rpc.getProof(ids[j]);
            expect(p.exists(ids[j])).toBeTruthy();
        }
    });

    it('should refuse a proof for a non-existing key', async () => {
        const badKey = Buffer.from('123');
        const badProof = await rpc.getProof(badKey);
        expect(badProof.exists(Buffer.from(badKey))).toBeFalsy();
    });
});

describe('Proof Offline Tests', () => {
    it('should throw for invalid proofs', () => {
        let key = Buffer.from([]);

        const p1 = Proof.fromObject({ inclusionproof: { interiors: [] } });
        expect(() => p1.exists(key)).toThrowError('key is nil');

        key = Buffer.from('123');
        expect(() => p1.exists(key)).toThrowError('no interior node');
    });
});
