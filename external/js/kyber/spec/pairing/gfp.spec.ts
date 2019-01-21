import BN = require('bn.js');
import GfP from '../../src/pairing/gfp';

describe('GfP', () => {
    it('should get the correct sign', () => {
        expect(new GfP(123).signum()).toBe(1);
        expect(new GfP(0).signum()).toBe(0);
        expect(new GfP(-123).signum()).toBe(-1);
    });

    it('should check zero and one', () => {
        const zero = new GfP(0);
        const one = new GfP(1);

        expect(zero.isZero()).toBeTruthy();
        expect(zero.isOne()).toBeFalsy();

        expect(one.isOne()).toBeTruthy();
        expect(one.isZero()).toBeFalsy();
    });

    it('should add and subtract', () => {
        const three = new GfP(1).add(new GfP(2));
        expect(three.equals(new GfP(3))).toBeTruthy();

        const one = new GfP(3).sub(new GfP(2));
        expect(one.equals(new GfP(1))).toBeTruthy();
    });

    it('should multiply and square', () => {
        const a = new GfP(123);

        expect(a.mul(a).mul(a).mul(a).equals(a.sqr().sqr())).toBeTruthy();
        expect(a.pow(new BN(3)).equals(a.sqr().mul(a))).toBeTruthy();
    });

    it('should compute the modulo', () => {
        const v = new GfP(-3);

        expect(v.mod(new BN(5)).getValue().toNumber()).toBe(2);
    });
});
