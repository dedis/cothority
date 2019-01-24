import BN from 'bn.js';
import { G1, G2, GT } from '../../src/pairing/bn';

describe('Optimal Ate Tests', () => {
    it('should get one', () => {
        const p = GT.pair(new G1(new BN(123)), new G2(new BN(456)));

        const a = new GT();
        a.neg(p);

        const one = new GT();
        one.add(a, p);
        expect(one.isOne()).toBeTruthy();
    });

    it('should pass the bilinearity test', () => {
        const v = '2c1660475bb9afe5a514d2ee8a2ff66e449024b0872a30e8d75a297cf6c82a0c79919ee0dd5618ecc6e89042b6ae7f74c9593b74e6e7ae344553af4578c0c6834e9421c990eff3660c4ca488a092eb9434b3c4a25f3585425b409064cc446748357c04ae026baee936e32d3a32489f1d9db346791b88641ef3ef5f2dcf3cebd423e23465a2c96e600ea83eb9cf3c5ffb50beb926560a569ee80d52e165ddcb94817cf8d696d2def79933dc0374ad1ac09b3f4834e17723374babde2f492473d41ca6856b6176795ba662de2f4a1208f1c3b3c5d4138929fa778d2aa2fcec7951457e039854ce6e3ebfcd75f317732abccfa233b5c6443d296bfaa5e7d6398c8d31db50c7ee4fe3ab79f311180711605a3f09f148edc5ffaf00b8bdc90a38702c301cd778cdbab48e375a783283759608a68bc933414f03f04083c12596b0d8ce798e7b670980dfe60a9fdbac4554455b4628e043696210da773b153433f0957b3245a9ba5b23ac3afecd786e692553f2ec42f7a2ff7a6bd4f204c4bf5d708831';
        const p1 = new G1(new BN(12345));
        const p2 = new G2(new BN(67890));

        const e1 = GT.pair(p1, p2);
        const e2 = GT.pair(new G1(new BN(1)), new G2(new BN(1)));
        e2.scalarMul(e2, new BN(12345));
        e2.scalarMul(e2, new BN(67890));

        expect(e1.marshal().toString('hex')).toBe(v);

        const minusE2 = new GT();
        minusE2.neg(e2);
        e1.add(e1, minusE2);

        expect(e1.isOne()).toBeTruthy();
    });

    it('should pair infinity points', () => {
        const p1 = new G1();
        p1.setInfinity();
        const p2 = new G2(new BN(123));

        const e1 = GT.pair(p1, p2);

        expect(e1.isOne()).toBeTruthy();
    });
});
