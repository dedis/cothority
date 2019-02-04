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
        expectAsync(rpc.getStatus()).toBeResolved();

        for (let i = 1; i < roster.length; i++) {
            expectAsync(rpc.getStatus(i)).toBeResolved();
        }

        expectAsync(rpc.getStatus(roster.length)).toBeRejected();
    });
});
