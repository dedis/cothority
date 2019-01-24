import BN from 'bn.js';
import jsc from 'jsverify';
import { sign, verify } from '../../../src/sign/bls';
import BN256Scalar from '../../../src/pairing/scalar';
import { BN256G2Point } from '../../../src/pairing/point';

describe('BLS Signature Tests', () => {
    const message = Buffer.from('test');
    const pub = '593c700babf825b6056a2339ce437f73f717226a77d618a5e8f0251c00273b38557c3cda8dbde5431d062804275f8757a2c942d888ac09f2df34f806e35e660a3c6f13dc64a7cf112865807450ccbd9f75bb3aadb98599f7034cf377a9b976045df374f840e9ee617631257fc9611def6c7c2e5cf23f5ab36cf72f68f14b6686';
    const secret = new BN256Scalar();
    secret.unmarshalBinary('008e886cf75be71a149322ddef13f81022f72d63a3d16d4d09cd0cf60dd8c9bc');

    it('should sign the message and verify the signature', () => {
        const sig = sign(message, secret);
        
        expect(sig.toString('hex')).toBe('8a82be45c20d81aa0c0ff319af108a61bf35e5aea2d17d0ead3b92fb1e77d22504dcab863dd0166539e75371efc4466d5f1645b45e7d29840547928cea382527');

        const p = new BN256G2Point();
        p.unmarshalBinary(Buffer.from(pub, 'hex'));

        expect(verify(message, p, sig)).toBeTruthy();
    });

    it('should not verify a wrong signature', () => {
        const sig = sign(Buffer.from('this is a message'), secret);
        const p = new BN256G2Point();
        p.unmarshalBinary(Buffer.from(pub, 'hex'));

        expect(verify(Buffer.from('this is another message'), p, sig)).toBeFalsy();
    });

    it('should pass the property-based check', () => {
        const prop = jsc.forall(jsc.string, jsc.array(jsc.nat), (msg, k) => {
            const message = Buffer.from(msg);
            const secret = new BN256Scalar(new BN(k));
            const pub = new BN256G2Point(secret.getValue());

            const sig = sign(message, secret);
            return verify(message, pub, sig);
        });

        // @ts-ignore
        expect(prop).toHold();
    });
});
