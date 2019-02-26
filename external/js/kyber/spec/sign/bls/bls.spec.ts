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
        
        // reference test with Go implementation
        expect(sig.toString('hex')).toBe('3bcac305c40c84155faa4c03899a2083cd601f6eb43ca7bd2e2b675311fd6fdc4295b573f6725e47a94d77015c89d81b562f6eba2c2d3755e8ce390af56688ca');

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
