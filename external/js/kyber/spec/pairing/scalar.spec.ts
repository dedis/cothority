import BN from 'bn.js';
import jsc from 'jsverify';
import BN256Scalar from '../../src/pairing/scalar';
import { p } from '../../src/pairing/constants';

describe('BN256 Scalar Tests', () => {
    it('should add', () => {
        const prop = jsc.forall(jsc.integer, jsc.integer, (a, b) => {
            const sA = new BN256Scalar(a);
            const sB = new BN256Scalar(b);
            const sum = new BN256Scalar();
            sum.add(sA, sB);
            sum.add(sum, new BN256Scalar().zero());

            return sum.getValue().eq(new BN(a + b).umod(p));
        });

        // @ts-ignore
        expect(prop).toHold();
    });

    it('should subtract', () => {
        const prop = jsc.forall(jsc.integer, jsc.integer, (a, b) => {
            const sA = new BN256Scalar(a);
            const sB = new BN256Scalar(b);
            const res = new BN256Scalar();
            res.sub(sA, sB);

            return res.getValue().eq(new BN(a - b).umod(p));
        });

        // @ts-ignore
        expect(prop).toHold();
    });

    it('should multiply', () => {
        const prop = jsc.forall(jsc.integer, jsc.integer, (a, b) => {
            const sA = new BN256Scalar(a);
            const sB = new BN256Scalar(b);
            const res = new BN256Scalar();
            res.mul(sA, sB);

            return res.getValue().eq(new BN(a * b).umod(p));
        });

        // @ts-ignore
        expect(prop).toHold();
    });

    it('should divide', () => {
        const prop = jsc.forall(jsc.nat, jsc.nat, (a, b) => {
            const sA = new BN256Scalar(a*(b+1));
            const sB = new BN256Scalar(b+1);
            const res = new BN256Scalar();
            res.div(sA, sB);

            return res.getValue().eq(new BN(a).umod(p));
        });

        // @ts-ignore
        expect(prop).toHold();
    });

    it('should get the negative', () => {
        const a = new BN256Scalar(-1);
        const n = new BN256Scalar().neg(a);

        expect(n.equals(new BN256Scalar().one()))
    });

    it('should get the inverse', () => {
        const a = new BN256Scalar(123);
        const inv = new BN256Scalar().inv(a);

        const one = new BN256Scalar().mul(a, inv);
        expect(one.equals(new BN256Scalar().one())).toBeTruthy();
    });

    it('should marshal and unmarshal', () => {
        const prop = jsc.forall(jsc.integer, (num) => {
            const a = new BN256Scalar(num);
            const buf = a.marshalBinary();

            const b = new BN256Scalar();
            b.unmarshalBinary(buf);

            return a.equals(b);
        });

        // @ts-ignore
        expect(prop).toHold();
    });

    it('should get a random scalar', () => {
        for (let i = 0; i < 100; i++) {
            const a = new BN256Scalar().pick();
            const b = new BN256Scalar().pick();

            expect(a.equals(b)).toBeFalsy();
        }
    });

    it('should clone', () => {
        const a = new BN256Scalar(123);
        const b = new BN256Scalar().set(a);

        expect(a.clone().equals(a)).toBeTruthy();
        expect(a.equals(b)).toBeTruthy();
    });
});
