import BN from 'bn.js';
import jsc from 'jsverify';
import { BN256G2Point } from "../../../src/pairing/point";
import BN256Scalar from "../../../src/pairing/scalar";
import { Mask } from '../../../src/sign';
import { aggregateSignatures, hashPointToR, sign, verify, aggregatePublicKeys } from "../../../src/sign/bdn";

describe('BDN signatures Test', () => {
    it('should hash to R', () => {
        // reference test from Kyber Go
        const p1 = new BN256G2Point().base();
        const p2 = new BN256G2Point().mul(new BN256Scalar(2), p1);
        const p3 = new BN256G2Point().mul(new BN256Scalar(3), p1);

        const coefs = hashPointToR([p1, p2, p3]);

        expect(coefs[0].toString('hex')).toBe('35b5b395f58aba3b192fb7e1e5f2abd3');
        expect(coefs[1].toString('hex')).toBe('14dcc79d46b09b93075266e47cd4b19e');
        expect(coefs[2].toString('hex')).toBe('933f6013eb3f654f9489d6d45ad04eaf');

        const mask = new Mask([p1, p2, p3], Buffer.from([7]));
        const agg = aggregatePublicKeys(mask);
        const ref = '1432ef60379c6549f7e0dbaf289cb45487c9d7da91fc20648f319a9fbebb23164abea76cdf7b1a3d20d539d9fe096b1d6fb3ee31bf1d426cd4a0d09d603b09f55f473fde972aa27aa991c249e890c1e4a678d470592dd09782d0fb3774834f0b2e20074a49870f039848a6b1aff95e1a1f8170163c77098e1f3530744d1826ce';
        expect(agg.marshalBinary().toString('hex')).toBe(ref);
    });

    it('should refuse to aggregate mismatching arrays', () => {
        const pk1 = new BN256G2Point();
        const pk2 = new BN256G2Point();
        const mask = new Mask([pk1, pk2], Buffer.from([0b11]));

        expect(() => aggregateSignatures(mask, [])).toThrow();
    });

    function testAggregateSignature(msk) {
        const sk1 = new BN256Scalar().pick();
        const pk1 = new BN256G2Point(sk1.getValue());
        const sk2 = new BN256Scalar().pick();
        const pk2 = new BN256G2Point(sk2.getValue());
        const sk3 = new BN256Scalar().pick();
        const pk3 = new BN256G2Point(sk3.getValue());

        const msg = Buffer.from('abc');
        const wrongMsg = Buffer.from('ab');
        const mask = new Mask([pk1, pk2, pk3], msk);
        const sig = aggregateSignatures(mask, [
            // simple test if the sig is null and the peer not participating
            mask.isIndexEnabled(0) ? sign(msg, sk1) : null,
            sign(msg, sk2),
            sign(msg, sk3),
        ]).marshalBinary();

        expect(verify(msg, mask, sig)).toBeTruthy()
        expect(verify(wrongMsg, mask, sig)).toBeFalsy();
    }

    it('should verify different aggregate', () => {
        const vectors = [
            Buffer.from([0b1]),
            Buffer.from([0b10]),
            Buffer.from([0b100]),
            Buffer.from([0b11]),
            Buffer.from([0b101]),
            Buffer.from([0b111]),
        ];

        for (let i = 0; i < vectors.length; i++) {
            testAggregateSignature(vectors[i])
        }
    });

    it('should pass the property-based check', () => {
        const prop = jsc.forall(jsc.string, jsc.array(jsc.nat), jsc.array(jsc.nat), (msg, k, l) => {
            const message = Buffer.from(msg);
            const sk1 = new BN256Scalar(new BN(k));
            const pk1 = new BN256G2Point(sk1.getValue());
            const sk2 = new BN256Scalar(new BN(l));
            const pk2 = new BN256G2Point(sk2.getValue());
            const mask = new Mask([pk1, pk2], Buffer.from([3]))

            const sig = aggregateSignatures(mask, [sign(message, sk1), sign(message, sk2)]);

            return verify(message, mask, sig.marshalBinary());
        });

        // @ts-ignore
        expect(prop).toHold();
    });
});
