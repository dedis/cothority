import { sign, verify } from '../../../src/sign/anon';
import { curve } from '../../../src';

const ed25519 = curve.newCurve('edwards25519');

describe('Ring Signature Tests', () => {
    it('should sign and verify with a link', async () => {
        const signers = [0, 0, 0, 0, 0].map(() => {
            const secret = ed25519.scalar().pick();
            const pub = ed25519.point().base();
            pub.mul(secret, pub);

            return { pub, secret };
        });
        const msg = Buffer.from('deadbeef');
        const secret = signers[0].secret;
        const iid = Buffer.from([3, 2, 1]);

        const sig = await sign(msg, signers.map(s => s.pub), secret, iid);
        
        expect(verify(msg, signers.map(s => s.pub), sig.encode(), iid)).toBeTruthy();
        expect(verify(msg, signers.map(s => s.pub), sig.encode(), iid.slice(1))).toBeFalsy();
    });

    it('should sign and verify without a link', async () => {
        const msg = Buffer.from('deadbeef');
        const signers = [0, 0, 0, 0, 0].map(() => {
            const secret = ed25519.scalar().pick();
            const pub = ed25519.point().base();
            pub.mul(secret, pub);

            return { pub, secret };
        });

        for (const signer of signers) {
            const { secret } = signer;
            const sig = await sign(msg, signers.map(s => s.pub), secret);
            
            expect(verify(msg, signers.map(s => s.pub), sig.encode())).toBeTruthy();
        }
    });
});
