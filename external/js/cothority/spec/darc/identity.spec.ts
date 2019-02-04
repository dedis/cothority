import IdentityDarc from '../../src/darc/identity-darc';
import { SIGNER } from '../support/conondes';
import IdentityEd25519 from '../../src/darc/identity-ed25519';

describe('Identity Tests', () => {
    it('should create a darc identity', () => {
        const id = new IdentityDarc({ id: Buffer.from('deadbeef', 'hex') });

        expect(id.verify(Buffer.from([]), Buffer.from([]))).toBeFalsy();
        expect(id.toWrapper().darc).toBeDefined();
        expect(id.toBytes()).toEqual(Buffer.from('deadbeef', 'hex'));
        expect(id.toString()).toBe('darc:deadbeef');
    });

    it('should create a ed25519 identity', () => {
        const id = new IdentityEd25519({ point: SIGNER.point });

        const msg = Buffer.from('deadbeef', 'hex');
        const sig = SIGNER.sign(msg);
        expect(id.verify(msg, sig.signature)).toBeTruthy();
        expect(id.toWrapper().ed25519).toBeDefined();
        expect(id.toBytes()).toEqual(SIGNER.point);
        expect(id.toString()).toBe(`ed25519:${SIGNER.public.toString()}`);
    });
});
