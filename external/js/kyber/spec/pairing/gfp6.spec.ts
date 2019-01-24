import GfP6 from '../../src/pairing/gfp6';
import GfP2 from '../../src/pairing/gfp2';

describe('GfP6', () => {
    const a = new GfP6(
        new GfP2("239487238491", "2356249827341"),
        new GfP2("082659782", "182703523765"),
        new GfP2("978236549263", "64893242"),
    );

    it('should invert', () => {
        const inv = a.invert();
        const b = inv.mul(a);

        expect(a.equals(a.invert().invert())).toBeTruthy();
        expect(b.isOne()).toBeTruthy();

        const one = GfP6.one();
        expect(one.invert().equals(one)).toBeTruthy();
    });

    it('should add and subtract', () => {
        const b = a.add(a);
        const c = a.neg();

        expect(b.add(c).equals(a)).toBeTruthy();
        expect(b.sub(a).equals(a)).toBeTruthy();
    });

    it('should square and mul', () => {
        const s = a.square().square();
        const m = a.mul(a).mul(a).mul(a);

        expect(s.equals(a)).toBeFalsy();
        expect(s.equals(m)).toBeTruthy();
    });

    it('should negate', () => {
        const n = a.neg();

        expect(n.equals(a)).toBeFalsy();
        expect(n.neg().equals(a)).toBeTruthy();
    });
});
