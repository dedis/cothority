import fs from 'fs';
import StatusRPC from '../../src/status/status-rpc';
import { Roster } from '../../src/network/roster';

const data = fs.readFileSync(process.cwd() + '/spec/support/public.toml');

describe('StatusRPC', () => {
    const roster = Roster.fromTOML(data);

    it('should get the status of the conode', async () => {
        const rpc = new StatusRPC(roster);

        const res = await rpc.getStatus();
        //console.log(res.toString());

        expect(res).toBeDefined();
    });
});
