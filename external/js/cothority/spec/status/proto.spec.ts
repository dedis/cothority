import { Status, StatusResponse } from "../../src/status/proto";
import { ServerIdentity } from "../../src/network/proto";

describe('Status Proto Tests', () => {
    it('should get specific values', () => {
        const status = new Status({ field: { a: 'a', b: 'b' } });

        expect(status.getValue('a')).toBe('a');
        expect(status.getValue('b')).toBe('b');
        expect(status.getValue('c')).toBeUndefined();
        expect(status.toString()).toBeDefined();
    });

    it('should get the status of a service', () => {
        const res = new StatusResponse({
            status: { 'service': new Status() },
            serveridentity: new ServerIdentity({ id: Buffer.from([]) }),
        });

        expect(res.getStatus('service')).toBeDefined();
        expect(res.serverIdentity).toBeDefined();
        expect(res.toString()).toBeDefined();
    });
});
