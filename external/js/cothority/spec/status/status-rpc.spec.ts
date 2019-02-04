import StatusRPC from '../../src/status/status-rpc';
import { startConodes, ROSTER } from '../support/conondes';

describe('StatusRPC', () => {
    const roster = ROSTER.slice(0, 4);

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
