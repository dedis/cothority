import fs from 'fs';
import SkipchainRPC from '../../src/skipchain/skipchain-rpc';
import { Roster } from '../../src/network/proto';
import { SkipBlock } from '../../src/skipchain/skipblock';
import { startConodes } from '../support/conondes';

const data = fs.readFileSync(process.cwd() + '/spec/support/public.toml');

describe('SkipchainRPC Tests', () => {
    const roster = Roster.fromTOML(data).slice(0, 4);

    beforeAll(async () => {
        await startConodes();
    }, 30 * 1000);

    it('should create a skipchain and add blocks to it', async () => {
        const rpc = new SkipchainRPC(roster);

        const { latest: genesis } = await rpc.createSkipchain();

        for (let i = 0; i < 10; i++) {
            await rpc.addBlock(genesis.hash, Buffer.from('abc'));
        }

        const latest = await rpc.getLatestBlock(genesis.hash);

        expect(latest.index).toBe(10);
        expect(latest.data.toString()).toBe('abc');

        const block = await rpc.getSkipblock(genesis.hash);
        expect(block.forward.length).toBeGreaterThan(1);

        const update = await rpc.getLatestBlock(genesis.hash);
        expect(update.hash).toEqual(latest.hash);
    });

    it('should fail to get the block', async () => {
        const rpc = new SkipchainRPC(roster);

        let err: Error;
        try {
            await rpc.getSkipblock(Buffer.from([1, 2, 3]));
        } catch (e) {
            err = e;
        }

        expect(err).toBeDefined();
        expect(err.message).toBe('No such block');
    });

    it('should verify the chain', () => {
        const rpc = new SkipchainRPC(roster);
        const blocks: SkipBlock[] = [];

        expect(rpc.verifyChain(blocks).message).toContain('No block');
    });
});
