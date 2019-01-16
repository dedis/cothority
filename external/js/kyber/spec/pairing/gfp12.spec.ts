import BN = require('bn.js');
import GfP12 from '../../src/pairing/gfp12';
import GfP6 from '../../src/pairing/gfp6';
import GfP2 from '../../src/pairing/gfp2';

describe('GfP12', () => {
    const a = new GfP12(
        new GfP6(
            new GfP2("239846234862342323958623", "2359862352529835623"),
            new GfP2("928836523", "9856234"),
            new GfP2("235635286", "5628392833"),
        ),
        new GfP6(
            new GfP2("252936598265329856238956532167968", "23596239865236954178968"),
            new GfP2("95421692834", "236548"),
            new GfP2("924523", "12954623"),
        ),
    );

    it('should generate one and zero', () => {
        const one = GfP12.one();
        const zero = GfP12.zero();

        expect(one.isOne()).toBeTruthy();
        expect(one.isZero()).toBeFalsy();

        expect(zero.isZero()).toBeTruthy();
        expect(zero.isOne()).toBeFalsy();
    });

    it('should invert', () => {
        const inv = a.invert();
        const b = inv.mul(a);

        expect(b.equals(GfP12.one())).toBeTruthy();
        expect(inv.invert().equals(a)).toBeTruthy();
    });

    it('should square and multiply', () => {
        const s = a.square().square();
        const m = a.mul(a).mul(a).mul(a);
        const e = a.exp(new BN(4));

        expect(s.equals(m)).toBeTruthy();
        expect(s.equals(e)).toBeTruthy();
    });

    it('should add and subtract', () => {
        const aa = a.add(a);
        
        expect(aa.equals(a)).toBeFalsy();
        expect(aa.sub(a).equals(a)).toBeTruthy();
    });

    it('should get the negative and conjugate', () => {
        const n = a.neg();
        const c = a.conjugate();

        expect(n.neg().equals(a)).toBeTruthy();
        expect(c.conjugate().equals(a)).toBeTruthy();
    });

    it('should stringify', () => {
        const one = GfP12.one();

        expect(one.toString()).toBe('(((0,0), (0,0), (0,0)), ((0,0), (0,0), (0,1)))');
    });
});
