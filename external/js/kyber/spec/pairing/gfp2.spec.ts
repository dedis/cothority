import GfP2 from '../../src/pairing/gfp2';
import GfP from '../../src/pairing/gfp';

describe('GfP2', () => {
    it('should generate one and zero', () => {
        const one = GfP2.one();
        const zero = GfP2.zero();

        expect(one.isOne()).toBeTruthy();
        expect(zero.isOne()).toBeFalsy();
        
        expect(zero.isZero()).toBeTruthy();
        expect(one.isZero()).toBeFalsy();
    });

    it('should invert', () => {
        const a = new GfP2('23423492374', '12934872398472394827398479');
        
        const inv = a.invert();
        expect(a.equals(inv)).toBeFalsy();
        expect(inv.invert().equals(a)).toBeTruthy();
        expect(inv.mul(a).equals(GfP2.one())).toBeTruthy();
    });

    it('should get the conjugate', () => {
        const a = new GfP2('23423492374', '12934872398472394827398479');

        const c = a.conjugate();
        expect(c.equals(a)).toBeFalsy();
        expect(c.conjugate().equals(a)).toBeTruthy();
    });

    it('should get the negative', () => {
        const a = new GfP2('23423492374', '12934872398472394827398479');
        const n = a.negative();

        expect(a.equals(n)).toBeFalsy();
        expect(n.negative().equals(a)).toBeTruthy();
    });

    it('should square', () => {
        const a = new GfP2('23423492374', '12934872398472394827398479');
        const s = a.square();
        const m = a.mul(a);

        expect(s.equals(m)).toBeTruthy();
    });

    it('should multiply by a scalar', () => {
        const a = new GfP2('23423492374', '12934872398472394827398479');
        const b = a.mulScalar(new GfP(3));
        const c = a.add(a).add(a);

        expect(b.equals(c)).toBeTruthy();
    });

    it('should subtract', () => {
        const a = new GfP2('23423492374', '12934872398472394827398479');
        const b = a.mulScalar(new GfP(2)).sub(a);

        expect(a.equals(b)).toBeTruthy();
    });

    it('should stringify', () => {
        const one = GfP2.one();

        expect(one.toString()).toBe('(0,1)');
    });
});
