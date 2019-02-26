import jsc from 'jsverify';
import { BN256G1Point, BN256G2Point } from '../../src/pairing/point';
import BN256Scalar from '../../src/pairing/scalar';
import { order } from '../../src/pairing/constants';

describe('BN256 Point Tests', () => {
    it('should get the order of g1', () => {
        const a = new BN256G1Point();
        a.mul(new BN256Scalar(order), new BN256G1Point().base());

        expect(a.equals(new BN256G1Point().null())).toBeTruthy();
    });

    it('should add and subtract g1 points', () => {
        const prop = jsc.forall(jsc.array(jsc.uint8), jsc.array(jsc.uint8), (a, b) => {
            const p1 = new BN256G1Point(a);
            const p2 = new BN256G1Point(b);

            const aa = new BN256G1Point().add(p1, p2)
            const bb = new BN256G1Point().sub(p1, p2.clone().neg(p2));

            return aa.equals(bb);
        });

        // @ts-ignore
        expect(prop).toHold();
    });

    it('should add and multiply g1 points', () => {
        const prop = jsc.forall(jsc.array(jsc.uint8), (a) => {
            const p1 = new BN256G1Point(a);

            const aa = new BN256G1Point().mul(new BN256Scalar(3), p1);
            const bb = new BN256G1Point().add(p1, p1);
            bb.add(bb, p1);

            return aa.equals(bb);
        });

        // @ts-ignore
        expect(prop).toHold();
    });

    it('should marshal and unmarshal g1 points', () => {
        const prop = jsc.forall(jsc.array(jsc.uint8), (a) => {
            const p1 = new BN256G1Point(a);

            const buf = p1.marshalBinary();
            const p2 = new BN256G1Point();
            p2.unmarshalBinary(buf);

            return p1.equals(p2) && p2.marshalSize() === buf.length;
        });

        // @ts-ignore
        expect(prop).toHold();
    });

    // Test written because of the edge case found by the property-based
    // test
    it('should marshal and unmarshal g1 point generated with k=1', () => {
        const p1 = new BN256G1Point([1]);

        const buf = p1.marshalBinary();
        const p2 = new BN256G1Point();
        p2.unmarshalBinary(buf);

        expect(p1.equals(p2)).toBeTruthy();
        expect(p2.marshalSize()).toBe(buf.length);
    });

    it('should get random g1', () => {
        for (let i = 0; i < 100; i++) {
            const a = new BN256G1Point().pick();
            const b = new BN256G1Point().pick();

            expect(a.equals(b)).toBeFalsy();
        }
    });

    it('should hash the message to a point', () => {
        const p1 = BN256G1Point.hashToPoint(Buffer.from('abc'));
        const ref1 = Buffer.from('2ac314dc445e47f096d15425fc294601c1a7d8d27561c4fe9bb452f593f77f4705230e9663123b93c06ce0cd49a893619a92019566f326829a39d6f5ce10579d', 'hex');
        expect(p1.marshalBinary().equals(ref1)).toBeTruthy();

        const p2 = BN256G1Point.hashToPoint(Buffer.from('e0a05cbb37fd6c159732a8c57b981773f7480695328b674d8a9cc083377f1811', 'hex'));
        const ref2 = Buffer.from('1444853e16a3f959e9ff1da9c226958f9ee4067f82451bcf88ecc5980cf2c4d50095605d82d456fbb24b21f283842746935e0c42c7f7a8f579894d9bccede5ae', 'hex');
        expect(p2.marshalBinary().equals(ref2)).toBeTruthy();
    });

    it('should get the string representation of G1', () => {
        const a = new BN256G1Point().null();

        expect(a.toString()).toBe('bn256.G1(0,1)');
    });

    it('should add and subtract g2 points', () => {
        const prop = jsc.forall(jsc.array(jsc.uint8), jsc.array(jsc.uint8), (a, b) => {
            const p1 = new BN256G2Point(a);
            const p2 = new BN256G2Point(b);

            const aa = new BN256G2Point().add(p1, p2)
            const bb = new BN256G2Point().sub(p1, p2.clone().neg(p2));

            return aa.equals(bb);
        });

        // @ts-ignore
        expect(prop).toHold();
    });

    it('should add and subtract 0 and 1', () => {
        const p1 = new BN256G2Point([]);
        const p2 = new BN256G2Point([1]);

        expect(p1.getElement().isInfinity());

        const aa = new BN256G2Point().add(p1, p2);
        const bb = new BN256G2Point().sub(p1, p2.clone().neg(p2));

        expect(aa.equals(bb)).toBeTruthy();
    });

    it('should add and multiply g2 points', () => {
        const prop = jsc.forall(jsc.array(jsc.uint8), (a) => {
            const p1 = new BN256G2Point(a);

            const aa = new BN256G2Point().mul(new BN256Scalar(3), p1);
            const bb = new BN256G2Point().add(p1, p1);
            bb.add(bb, p1);

            return aa.equals(bb);
        });

        // @ts-ignore
        expect(prop).toHold();
    });

    it('should marshal and unmarshal g2 points', () => {
        const prop = jsc.forall(jsc.array(jsc.uint8), (a) => {
            const p1 = new BN256G2Point(a);

            const buf = p1.marshalBinary();
            const p2 = new BN256G2Point();
            p2.unmarshalBinary(buf);

            return p1.equals(p2) && p2.marshalSize() === buf.length;
        });

        // @ts-ignore
        expect(prop).toHold();
    });

    it('should get random g2', () => {
        for (let i = 0; i < 100; i++) {
            const a = new BN256G2Point().pick();
            const b = new BN256G2Point().pick();

            expect(a.equals(b)).toBeFalsy();
        }
    })

    it('should get the string representation of G2', () => {
        const a = new BN256G2Point().null();

        expect(a.toString()).toBe('bn256.G2((0,0),(0,1),(0,0))');
    });

    it('should pair g1 and g2 points', () => {
        const prop = jsc.forall(jsc.array(jsc.uint8), jsc.array(jsc.uint8), (a, b) => {
            const p1 = new BN256G1Point(a);
            const p2 = new BN256G2Point(b);

            const k1 = p1.pair(p2);
            const k2 = p2.pair(p1);

            return k1.equals(k2);
        });

        // @ts-ignore
        expect(prop).toHold();
    });

    it('should throw unimplemented errors', () => {
        const a = new BN256G1Point();

        expect(() => a.embedLen()).toThrow();
        expect(() => a.embed(Buffer.from([]))).toThrow();
        expect(() => a.data()).toThrow();

        const b = new BN256G2Point();

        expect(() => b.embedLen()).toThrow();
        expect(() => b.embed(Buffer.from([]))).toThrow();
        expect(() => b.data()).toThrow();
    });
});
