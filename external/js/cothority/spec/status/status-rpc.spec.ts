import fs from 'fs';
import StatusRPC from '../../src/status/status-rpc';
import { Roster } from '../../src/network/proto';
import { startConodes } from '../support/conondes';

const data = fs.readFileSync(process.cwd() + '/spec/support/public.toml');

describe('StatusRPC', () => {
    const roster = Roster.fromTOML(data);

    beforeAll(async () => {
        await startConodes();
    }, 30 * 1000);

    it('should get the status of the conode', async () => {
        const rpc = new StatusRPC(roster);

        expect(roster.length).toBeGreaterThan(0);

        for (let i = 0; i < roster.length; i++) {
            const res = await rpc.getStatus();

            expect(res).toBeDefined();
        }
    });
});
