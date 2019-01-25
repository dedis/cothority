import fs from 'fs';
import { Roster } from '../../src/network/proto';
import { startConodes } from '../support/conondes';
import ByzCoinRPC from '../../src/byzcoin/byzcoin-rpc';
import { IdentityEd25519 } from '../../src/darc/IdentityEd25519';
import { InstanceID } from '../../src/byzcoin/ClientTransaction';

const data = fs.readFileSync(process.cwd() + '/spec/support/public.toml');

describe('ByzCoinRPC Tests', () => {
    const roster = Roster.fromTOML(data).slice(0, 4);
    const admin = new IdentityEd25519({ point: Buffer.from('8032764c1ebb0bca31a32e6af3aed014ad89c050165cfd8ac85139f9f3d4d698', 'hex') });

    beforeAll(async () => {
        await startConodes();
    }, 30 * 1000);

    it('should create an rpc', async () => {
        const darc = ByzCoinRPC.makeGenesisDarc([admin], roster);
        console.log(darc.toString());
        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, 1000);

        const t = await rpc.getProof(new InstanceID(Buffer.alloc(32, 0)));
        console.log(t);
    }, 20* 1000);
});
